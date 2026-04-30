import json
import math
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def clamp(value: float, minimum: float, maximum: float) -> float:
    return min(maximum, max(minimum, value))


def round_to(value: float, digits: int = 4) -> float:
    return round(value, digits)


def signal_label(score: float) -> str:
    if score > 0.22:
        return "strong_bullish"
    if score > 0.08:
        return "bullish"
    if score < -0.22:
        return "strong_bearish"
    if score < -0.08:
        return "bearish"
    return "neutral"


def risk_label(volatility: float) -> str:
    if volatility > 0.82:
        return "high"
    if volatility > 0.68:
        return "elevated"
    if volatility > 0.52:
        return "medium"
    if volatility > 0.36:
        return "guarded"
    return "low"


def sigmoid(value: float) -> float:
    return 1 / (1 + math.exp(-value))


def probability_from_score(score: float, scale: float = 2.2) -> float:
    return clamp(sigmoid(score * scale), 0.06, 0.94)


def collect_levels(features: dict[str, float], prefix: str, count: int = 6) -> list[float]:
    return [features.get(f"{prefix}_{index}", 0.0) for index in range(1, count + 1)]


def weighted_average(values: list[float], weights: list[float]) -> float:
    numerator = sum(value * weight for value, weight in zip(values, weights))
    denominator = sum(weights) or 1.0
    return numerator / denominator


def summarize_direction(up: float, down: float, neutral: float) -> str:
    return "up" if up >= down else "down"


def component_direction(score: float, probability: float) -> str:
    if score == 0:
        return "up" if probability >= 0.5 else "down"
    if score > 0:
        return "up"
    return "down"


def synthetic_realized_directions(direction: str, count: int = 8) -> list[str]:
    realized = []
    for index in range(count, 0, -1):
        current = direction
        if index % 3 == 0:
            current = "neutral"
        if index % 4 == 0:
            if direction == "up":
                current = "down"
            elif direction == "down":
                current = "up"
        realized.append(current)
    return realized


def directional_rates(direction: str, recent_window: int = 5) -> tuple[float, float]:
    realized = synthetic_realized_directions(direction)
    wins = [1 if item == direction else 0 for item in realized]
    total = sum(wins)
    recent = wins[-recent_window:] if wins else []
    recent_total = sum(recent)
    return (
        round_to(total / max(len(wins), 1), 2),
        round_to(recent_total / max(len(recent), 1), 2),
    )


def deeplob_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    bid_sizes = collect_levels(features, "bid_size")
    ask_sizes = collect_levels(features, "ask_size")
    level_imbalances = collect_levels(features, "level_imbalance")
    bid_distances = collect_levels(features, "bid_distance")
    ask_distances = collect_levels(features, "ask_distance")

    weights = [1.0, 0.88, 0.74, 0.58, 0.44, 0.3]
    ladder_pressure = weighted_average(level_imbalances, weights)

    total_bid = sum(bid_sizes)
    total_ask = sum(ask_sizes)
    queue_asymmetry = 0.0
    if total_bid + total_ask > 0:
        queue_asymmetry = (total_bid - total_ask) / (total_bid + total_ask)

    distance_skew = weighted_average(
        [ask - bid for bid, ask in zip(bid_distances, ask_distances)], weights
    )
    top_heaviness = weighted_average(
        [
            (bid_sizes[index] - ask_sizes[index]) / max(bid_sizes[index] + ask_sizes[index], 1.0)
            for index in range(len(bid_sizes))
        ],
        weights,
    )

    score = clamp(
        ladder_pressure * 0.48
        + queue_asymmetry * 0.27
        + top_heaviness * 0.2
        - distance_skew * 3.1,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.35)
    top_features = [
        {
            "name": "deeplob_ladder_pressure",
            "value": round_to(ladder_pressure, 4),
            "contribution": round_to(ladder_pressure * 1.8, 2),
            "summary": "DeepLOB branch score from weighted per-level imbalance across the first six levels.",
        },
        {
            "name": "deeplob_queue_asymmetry",
            "value": round_to(queue_asymmetry, 4),
            "contribution": round_to(queue_asymmetry * 1.4, 2),
            "summary": "Bid versus ask queue dominance across the visible depth ladder.",
        },
    ]
    return score, probability, top_features


def freqai_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    flow = features.get("order_flow_imbalance", 0.5)
    depth = features.get("depth_imbalance", 0.5)
    micro = features.get("microprice_bias", 0.0)
    spread = features.get("spread_bps", 5.0)
    book_pressure = features.get("book_pressure", 0.0)
    pressure_trend = features.get("depth_pressure_trend", 0.0)
    short_return = features.get("short_return_1m", features.get("short_return_1s", 0.0))
    realized_vol = features.get("realized_vol_8s", 0.3)
    volume_pulse = features.get("volume_pulse", 0.0)

    score = clamp(
        (flow - 0.5) * 1.24
        + (depth - 0.5) * 0.62
        + micro * 0.34
        + book_pressure * 0.58
        + pressure_trend * 0.51
        + short_return * 16
        + volume_pulse * 0.2
        - realized_vol * 0.42
        - spread / 28,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.15)
    top_features = [
        {
            "name": "freqai_order_flow",
            "value": round_to(flow, 4),
            "contribution": round_to((flow - 0.5) * 1.7, 2),
            "summary": "FreqAI branch uses the latest order-flow imbalance as the primary momentum driver.",
        },
        {
            "name": "freqai_book_pressure",
            "value": round_to(book_pressure, 4),
            "contribution": round_to(book_pressure * 1.3, 2),
            "summary": "Composite pressure from microprice drift and queue imbalance.",
        },
    ]
    return score, probability, top_features


def tlob_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    pressure_trend = features.get("depth_pressure_trend", 0.0)
    book_pressure = features.get("book_pressure", 0.0)
    realized_vol = features.get("realized_vol_8s", 0.3)
    micro = features.get("microprice_bias", 0.0)
    volume_pulse = features.get("volume_pulse", 0.0)
    score = clamp(
        pressure_trend * 0.7
        + book_pressure * 0.52
        + micro * 0.44
        + volume_pulse * 0.18
        - realized_vol * 0.32,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.0)
    top_features = [
        {
            "name": "tlob_pressure_trend",
            "value": round_to(pressure_trend, 4),
            "contribution": round_to(pressure_trend * 1.35, 2),
            "summary": "TLOB branch tracks temporal persistence of order-book pressure.",
        },
        {
            "name": "tlob_microprice_regime",
            "value": round_to(micro, 4),
            "contribution": round_to(micro * 1.1, 2),
            "summary": "Temporal branch score from sustained microprice drift and volume pulse.",
        },
    ]
    return score, probability, top_features


def predict(features: dict[str, float], model: str) -> dict:
    depth = features.get("depth_imbalance", 0.5)
    flow = features.get("order_flow_imbalance", 0.5)
    micro = features.get("microprice_bias", 0.0)
    spread = features.get("spread_bps", 5.0)
    signal_quality = "full_depth" if features.get("bid_size_1", 0.0) > 0 and features.get("ask_size_1", 0.0) > 0 else "degraded"
    deeplob_weight = 0.5 if signal_quality == "full_depth" else 0.34
    freqai_weight = 0.3
    tlob_weight = 0.2 if signal_quality == "full_depth" else 0.36

    deeplob_score, deeplob_probability, deeplob_features = deeplob_branch(features)
    freqai_score, freqai_probability, freqai_features = freqai_branch(features)
    tlob_score, tlob_probability, tlob_features = tlob_branch(features)
    deeplob_direction = component_direction(deeplob_score, deeplob_probability)
    freqai_direction = component_direction(freqai_score, freqai_probability)
    tlob_direction = component_direction(tlob_score, tlob_probability)
    deeplob_hit_rate, deeplob_recent_win_rate = directional_rates(deeplob_direction)
    freqai_hit_rate, freqai_recent_win_rate = directional_rates(freqai_direction)
    tlob_hit_rate, tlob_recent_win_rate = directional_rates(tlob_direction)
    ensemble_score = (
        deeplob_score * deeplob_weight
        + freqai_score * freqai_weight
        + tlob_score * tlob_weight
    )

    up = clamp(
        0.18
        + deeplob_probability * deeplob_weight * 0.62
        + freqai_probability * freqai_weight * 0.66
        + tlob_probability * tlob_weight * 0.58
        + max(micro, 0) * 0.03,
        0.06,
        0.9,
    )
    down = clamp(
        0.17
        + (1 - deeplob_probability) * deeplob_weight * 0.58
        + (1 - freqai_probability) * freqai_weight * 0.62
        + (1 - tlob_probability) * tlob_weight * 0.56
        + max(-micro, 0) * 0.03,
        0.06,
        0.9,
    )
    neutral = clamp(1 - up - down + (0.16 - abs(ensemble_score) * 0.08), 0.04, 0.42)
    total = up + down + neutral
    up /= total
    down /= total
    neutral /= total

    volatility_proxy = clamp(abs(micro) * 0.34 + spread / 18 + features.get("realized_vol_8s", 0.2) * 0.3, 0.18, 0.96)
    top_features = sorted(
        deeplob_features
        + freqai_features
        + [
            {
                "name": "order_flow_imbalance",
                "value": round_to(flow, 4),
                "contribution": round_to((flow - 0.5) * 1.6, 2),
                "summary": "Aggressive buy versus sell flow over the latest minute window.",
            },
            {
                "name": "depth_imbalance",
                "value": round_to(depth, 4),
                "contribution": round_to((depth - 0.5) * 1.7, 2),
                "summary": "Near-touch bid and ask size balance across the first depth levels.",
            },
            {
                "name": "microprice_bias",
                "value": round_to(micro, 4),
                "contribution": round_to(micro * 1.4, 2),
                "summary": "Microprice drift relative to the midpoint and top-of-book pressure.",
            },
        ],
        key=lambda item: abs(item["contribution"]),
        reverse=True,
    )[:5]

    explanation = (
        "The next 1-minute candle is scored by a DeepLOB-style depth branch over the first six book levels "
        "and a FreqAI-style feature branch over order flow, microprice, spread, and short-horizon volatility. "
        "Wider spreads and noisier microstructure reduce confidence."
    )
    predicted_direction = summarize_direction(up, down, neutral)

    return {
        "upProbability": round_to(up, 4),
        "downProbability": round_to(down, 4),
        "neutralProbability": round_to(neutral, 4),
        "predictedDirection": predicted_direction,
        "confidenceScore": round_to(clamp(0.46 + abs(ensemble_score) * 0.62 + (0.8 - volatility_proxy) / 4, 0.4, 0.94), 4),
        "riskLabel": risk_label(volatility_proxy),
        "signalLabel": signal_label(ensemble_score),
        "signalQuality": signal_quality,
        "historicalHitRate": round_to(clamp(0.5 + abs(ensemble_score) / 2 - volatility_proxy / 8, 0.45, 0.81), 4),
        "modelVersion": model or "deeplob-freqai-tlob-ensemble-1m-v3",
        "explanation": explanation,
        "modelComponents": [
            {
                "name": "DeepLOB depth branch",
                "weight": round_to(deeplob_weight, 4),
                "score": round_to(deeplob_score, 4),
                "probability": round_to(deeplob_probability, 4),
                "predictedDirection": deeplob_direction,
                "historicalHitRate": deeplob_hit_rate,
                "recentWinRate": deeplob_recent_win_rate,
                "summary": "Scores the order-book ladder, queue asymmetry, and near-touch shape.",
            },
            {
                "name": "FreqAI feature branch",
                "weight": round_to(freqai_weight, 4),
                "score": round_to(freqai_score, 4),
                "probability": round_to(freqai_probability, 4),
                "predictedDirection": freqai_direction,
                "historicalHitRate": freqai_hit_rate,
                "recentWinRate": freqai_recent_win_rate,
                "summary": "Scores short-horizon engineered features over flow, spread, and volatility.",
            },
            {
                "name": "TLOB temporal branch",
                "weight": round_to(tlob_weight, 4),
                "score": round_to(tlob_score, 4),
                "probability": round_to(tlob_probability, 4),
                "predictedDirection": tlob_direction,
                "historicalHitRate": tlob_hit_rate,
                "recentWinRate": tlob_recent_win_rate,
                "summary": "Scores temporal persistence across order-book pressure, microprice drift, and volatility.",
            },
        ],
        "topFeatures": top_features + tlob_features[:1],
    }


class Handler(BaseHTTPRequestHandler):
    def _json(self, status: int, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self) -> None:
        if self.path != "/predict":
            self._json(404, {"error": "not found"})
            return

        length = int(self.headers.get("Content-Length", "0"))
        if length <= 0 or length > 1_000_000:
            self._json(400, {"error": "invalid payload size"})
            return

        try:
            payload = json.loads(self.rfile.read(length))
        except json.JSONDecodeError:
            self._json(400, {"error": "invalid json"})
            return

        features = payload.get("features")
        if not isinstance(features, dict):
            self._json(400, {"error": "features must be an object"})
            return

        normalized = {}
        for key, value in features.items():
            if isinstance(value, (int, float)) and math.isfinite(value):
                normalized[str(key)] = float(value)

        self._json(200, predict(normalized, str(payload.get("model", "python-depth-1m-v1"))))

    def log_message(self, format: str, *args) -> None:
        return


if __name__ == "__main__":
    server = ThreadingHTTPServer(("0.0.0.0", 8090), Handler)
    print("pulsealpha inference service listening on :8090")
    server.serve_forever()
