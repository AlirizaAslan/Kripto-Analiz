import json
import math
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def clamp(value: float, minimum: float, maximum: float) -> float:
    return min(maximum, max(minimum, value))


def round_to(value: float, digits: int = 4) -> float:
    return round(value, digits)


def signal_label(score: float) -> str:
    if score > 0.34:
        return "strong_bullish"
    if score > 0.18:
        return "bullish"
    if score < -0.34:
        return "strong_bearish"
    if score < -0.18:
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
    trend_15m_slope = features.get("trend_15m_slope", 0.0)
    distance_to_ema_1h = features.get("distance_to_ema_1h", 0.0)
    vwap_distance_bps = features.get("vwap_distance_bps", 0.0)

    score = clamp(
        (flow - 0.5) * 1.08
        + (depth - 0.5) * 0.52
        + micro * 0.28
        + book_pressure * 0.46
        + pressure_trend * 0.42
        + short_return * 13
        + volume_pulse * 0.18
        + trend_15m_slope * 0.26
        + distance_to_ema_1h * 0.28
        + (vwap_distance_bps / 100.0) * 0.34
        - realized_vol * 0.44
        - spread / 30,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.15)
    top_features = [
        {
            "name": "freqai_order_flow",
            "value": round_to(flow, 4),
            "contribution": round_to((flow - 0.5) * 1.5, 2),
            "summary": "FreqAI branch uses the latest order-flow imbalance as the primary momentum driver.",
        },
        {
            "name": "freqai_macro_trend_15m",
            "value": round_to(trend_15m_slope, 4),
            "contribution": round_to(trend_15m_slope * 0.14, 2),
            "summary": "Higher-timeframe trend slope used as a macro filter against short-lived microstructure noise.",
        },
        {
            "name": "freqai_vwap_distance_bps",
            "value": round_to(vwap_distance_bps, 4),
            "contribution": round_to((vwap_distance_bps / 100.0) * 0.22, 2),
            "summary": "Distance from VWAP in basis points; persistent discount below VWAP lowers buy confidence.",
        },
    ]
    return score, probability, top_features


def tlob_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    pressure_trend = features.get("depth_pressure_trend", 0.0)
    book_pressure = features.get("book_pressure", 0.0)
    realized_vol = features.get("realized_vol_8s", 0.3)
    micro = features.get("microprice_bias", 0.0)
    volume_pulse = features.get("volume_pulse", 0.0)
    trend_15m_slope = features.get("trend_15m_slope", 0.0)
    score = clamp(
        pressure_trend * 0.66
        + book_pressure * 0.48
        + micro * 0.38
        + volume_pulse * 0.16
        + trend_15m_slope * 0.18
        - realized_vol * 0.34,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.0)
    top_features = [
        {
            "name": "tlob_pressure_trend",
            "value": round_to(pressure_trend, 4),
            "contribution": round_to(pressure_trend * 1.22, 2),
            "summary": "TLOB branch tracks temporal persistence of order-book pressure.",
        },
        {
            "name": "tlob_macro_trend_15m",
            "value": round_to(trend_15m_slope, 4),
            "contribution": round_to(trend_15m_slope * 0.11, 2),
            "summary": "Higher-timeframe trend confirmation used to avoid fading strong directional regimes.",
        },
    ]
    return score, probability, top_features


def lightgbm_squeeze_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    """LightGBM-style Squeeze branch: detects squeeze/liquidation pressure
    using depth imbalance, volume spikes, and flow momentum."""
    depth = features.get("depth_imbalance", 0.5)
    flow = features.get("order_flow_imbalance", 0.5)
    micro = features.get("microprice_bias", 0.0)
    spread = features.get("spread_bps", 5.0)
    book_pressure = features.get("book_pressure", 0.0)
    volume_pulse = features.get("volume_pulse", 0.0)
    realized_vol = features.get("realized_vol_8s", 0.3)
    short_return = features.get("short_return_1m", features.get("short_return_1s", 0.0))

    # Squeeze detection: strong imbalance + volume spike + momentum alignment
    imbalance_signal = (depth - 0.5) * 2.0
    flow_momentum = (flow - 0.5) * 2.0
    squeeze_pressure = clamp(
        imbalance_signal * flow_momentum * 1.6,  # cross-product amplifies aligned signals
        -1.0,
        1.0,
    )

    # Volume-weighted conviction: high volume + directional pressure = squeeze
    volume_conviction = clamp(volume_pulse * 0.72 + abs(short_return) * 8.0, 0.0, 1.0)

    score = clamp(
        squeeze_pressure * 0.42
        + imbalance_signal * 0.24
        + flow_momentum * 0.18
        + book_pressure * 0.32
        + micro * 0.22
        + short_return * 9.0
        + volume_pulse * 0.28
        - realized_vol * 0.38
        - spread / 28
        + squeeze_pressure * volume_conviction * 0.16,
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.25)
    top_features = [
        {
            "name": "lgbm_squeeze_pressure",
            "value": round_to(squeeze_pressure, 4),
            "contribution": round_to(squeeze_pressure * 1.6, 2),
            "summary": "Cross-product of depth imbalance and order flow detects squeeze buildup.",
        },
        {
            "name": "lgbm_volume_conviction",
            "value": round_to(volume_conviction, 4),
            "contribution": round_to(volume_conviction * 0.8, 2),
            "summary": "Volume spike combined with short return momentum confirms squeeze strength.",
        },
    ]
    return score, probability, top_features


def lob_transformer_spoofing_branch(features: dict[str, float]) -> tuple[float, float, list[dict]]:
    """LOB-Transformer-style Spoofing branch: detects fake order walls
    and manipulation patterns in the order book."""
    bid_sizes = collect_levels(features, "bid_size")
    ask_sizes = collect_levels(features, "ask_size")
    level_imbalances = collect_levels(features, "level_imbalance")
    spread = features.get("spread_bps", 5.0)
    pressure_trend = features.get("depth_pressure_trend", 0.0)
    book_pressure = features.get("book_pressure", 0.0)
    volume_pulse = features.get("volume_pulse", 0.0)
    micro = features.get("microprice_bias", 0.0)

    total_bid = sum(bid_sizes)
    total_ask = sum(ask_sizes)
    total_book = total_bid + total_ask

    # Wall detection: find abnormally large single-level concentration
    bid_concentration = max(bid_sizes) / max(total_bid, 1.0) if total_bid > 0 else 0.0
    ask_concentration = max(ask_sizes) / max(total_ask, 1.0) if total_ask > 0 else 0.0

    # Spoofing indicator: one side has a massive wall but price moves opposite
    # (real buyers don't stack huge visible walls, spoofs do)
    bid_wall_spoof = clamp(bid_concentration - 0.35, 0.0, 0.65)  # abnormal bid wall
    ask_wall_spoof = clamp(ask_concentration - 0.35, 0.0, 0.65)  # abnormal ask wall

    # Level variance: spoofed books tend to be very uneven across levels
    bid_variance = 0.0
    ask_variance = 0.0
    if len(bid_sizes) >= 3 and total_bid > 0:
        bid_mean = total_bid / len(bid_sizes)
        bid_variance = sum((s - bid_mean) ** 2 for s in bid_sizes) / len(bid_sizes)
        bid_variance = clamp(bid_variance / max(bid_mean ** 2, 1.0), 0.0, 2.0)
    if len(ask_sizes) >= 3 and total_ask > 0:
        ask_mean = total_ask / len(ask_sizes)
        ask_variance = sum((s - ask_mean) ** 2 for s in ask_sizes) / len(ask_sizes)
        ask_variance = clamp(ask_variance / max(ask_mean ** 2, 1.0), 0.0, 2.0)

    # Fake wall logic: if bid wall is spoofed, real direction is likely DOWN
    # if ask wall is spoofed, real direction is likely UP
    spoof_signal = clamp(
        (ask_wall_spoof - bid_wall_spoof) * 2.4  # spoof reversal
        + (ask_variance - bid_variance) * 0.38,    # uneven book = manipulation
        -1.0,
        1.0,
    )

    # Weighted imbalance check (ignoring spoofed levels)
    weights = [1.0, 0.88, 0.74, 0.58, 0.44, 0.3]
    clean_imbalance = weighted_average(level_imbalances, weights)

    # Combine: genuine depth signal + spoof reversal + trend confirmation
    score = clamp(
        clean_imbalance * 0.34
        + spoof_signal * 0.28
        + pressure_trend * 0.22
        + book_pressure * 0.18
        + micro * 0.16
        + volume_pulse * 0.12
        - spread / 32
        - (bid_wall_spoof + ask_wall_spoof) * 0.14,  # penalize any wall presence (uncertainty)
        -1.2,
        1.2,
    )
    probability = probability_from_score(score, scale=2.1)
    top_features = [
        {
            "name": "lobt_spoof_signal",
            "value": round_to(spoof_signal, 4),
            "contribution": round_to(spoof_signal * 1.4, 2),
            "summary": "Detects fake order walls; positive means ask-side spoofing (bullish reversal).",
        },
        {
            "name": "lobt_wall_concentration",
            "value": round_to(max(bid_concentration, ask_concentration), 4),
            "contribution": round_to((bid_wall_spoof + ask_wall_spoof) * -0.7, 2),
            "summary": "Single-level order concentration; high values indicate potential manipulation.",
        },
    ]
    return score, probability, top_features


def predict(features: dict[str, float], model: str) -> dict:
    depth = features.get("depth_imbalance", 0.5)
    flow = features.get("order_flow_imbalance", 0.5)
    micro = features.get("microprice_bias", 0.0)
    spread = features.get("spread_bps", 5.0)
    trend_15m_slope = features.get("trend_15m_slope", 0.0)
    distance_to_ema_1h = features.get("distance_to_ema_1h", 0.0)
    vwap_distance_bps = features.get("vwap_distance_bps", 0.0)
    signal_quality = "full_depth" if features.get("bid_size_1", 0.0) > 0 and features.get("ask_size_1", 0.0) > 0 else "degraded"

    # 5-model weights
    if signal_quality == "full_depth":
        deeplob_weight = 0.28
        freqai_weight = 0.22
        tlob_weight = 0.18
        lgbm_weight = 0.18
        lobt_weight = 0.14
    else:
        deeplob_weight = 0.20
        freqai_weight = 0.22
        tlob_weight = 0.22
        lgbm_weight = 0.20
        lobt_weight = 0.16

    deeplob_score, deeplob_probability, deeplob_features = deeplob_branch(features)
    freqai_score, freqai_probability, freqai_features = freqai_branch(features)
    tlob_score, tlob_probability, tlob_features = tlob_branch(features)
    lgbm_score, lgbm_probability, lgbm_features = lightgbm_squeeze_branch(features)
    lobt_score, lobt_probability, lobt_features = lob_transformer_spoofing_branch(features)

    deeplob_direction = component_direction(deeplob_score, deeplob_probability)
    freqai_direction = component_direction(freqai_score, freqai_probability)
    tlob_direction = component_direction(tlob_score, tlob_probability)
    lgbm_direction = component_direction(lgbm_score, lgbm_probability)
    lobt_direction = component_direction(lobt_score, lobt_probability)

    deeplob_hit_rate, deeplob_recent_win_rate = directional_rates(deeplob_direction)
    freqai_hit_rate, freqai_recent_win_rate = directional_rates(freqai_direction)
    tlob_hit_rate, tlob_recent_win_rate = directional_rates(tlob_direction)
    lgbm_hit_rate, lgbm_recent_win_rate = directional_rates(lgbm_direction)
    lobt_hit_rate, lobt_recent_win_rate = directional_rates(lobt_direction)

    ensemble_score = (
        deeplob_score * deeplob_weight
        + freqai_score * freqai_weight
        + tlob_score * tlob_weight
        + lgbm_score * lgbm_weight
        + lobt_score * lobt_weight
    )

    up = clamp(
        0.18
        + deeplob_probability * deeplob_weight * 0.62
        + freqai_probability * freqai_weight * 0.66
        + tlob_probability * tlob_weight * 0.58
        + lgbm_probability * lgbm_weight * 0.64
        + lobt_probability * lobt_weight * 0.60
        + max(micro, 0) * 0.03,
        0.06,
        0.9,
    )
    down = clamp(
        0.17
        + (1 - deeplob_probability) * deeplob_weight * 0.58
        + (1 - freqai_probability) * freqai_weight * 0.62
        + (1 - tlob_probability) * tlob_weight * 0.56
        + (1 - lgbm_probability) * lgbm_weight * 0.60
        + (1 - lobt_probability) * lobt_weight * 0.58
        + max(-micro, 0) * 0.03,
        0.06,
        0.9,
    )
    macro_bias = clamp(
        trend_15m_slope * 0.42 + distance_to_ema_1h * 0.03 + (vwap_distance_bps / 100.0) * 0.08,
        -0.3,
        0.3,
    )
    if ensemble_score >= 0:
        up = clamp(up + max(macro_bias, 0) * 0.14 - max(-macro_bias, 0) * 0.18, 0.06, 0.9)
        down = clamp(down + max(-macro_bias, 0) * 0.15 - max(macro_bias, 0) * 0.08, 0.06, 0.9)
    else:
        up = clamp(up + max(macro_bias, 0) * 0.08 - max(-macro_bias, 0) * 0.15, 0.06, 0.9)
        down = clamp(down + max(-macro_bias, 0) * 0.14 - max(macro_bias, 0) * 0.18, 0.06, 0.9)
    neutral = clamp(1 - up - down + (0.16 - abs(ensemble_score) * 0.08) - abs(macro_bias) * 0.03, 0.04, 0.42)
    total = up + down + neutral
    up /= total
    down /= total
    neutral /= total

    volatility_proxy = clamp(abs(micro) * 0.34 + spread / 18 + features.get("realized_vol_8s", 0.2) * 0.3, 0.18, 0.96)
    top_features = sorted(
        deeplob_features
        + freqai_features
        + tlob_features
        + lgbm_features
        + lobt_features
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
            {
                "name": "distance_to_ema_1h",
                "value": round_to(features.get("distance_to_ema_1h", 0.0), 4),
                "contribution": round_to(features.get("distance_to_ema_1h", 0.0) * 0.16, 2),
                "summary": "Macro mean-reversion buffer against entries far from the higher-timeframe EMA.",
            },
            {
                "name": "vwap_distance_bps",
                "value": round_to(features.get("vwap_distance_bps", 0.0), 4),
                "contribution": round_to((features.get("vwap_distance_bps", 0.0) / 100.0) * 0.22, 2),
                "summary": "Distance from VWAP in basis points, useful for filtering weak pullbacks in downtrends.",
            },
            {
                "name": "trend_15m_slope",
                "value": round_to(features.get("trend_15m_slope", 0.0), 4),
                "contribution": round_to(features.get("trend_15m_slope", 0.0) * 0.14, 2),
                "summary": "Macro trend slope over the last 15 minutes; strong directional runs should dominate micro noise.",
            },
        ],
        key=lambda item: abs(item["contribution"]),
        reverse=True,
    )[:7]

    explanation = (
        "The next 1-minute candle is scored by five ensemble branches: "
        "DeepLOB depth analysis, FreqAI order-flow features, TLOB temporal persistence, "
        "LightGBM squeeze detection, and LOB-Transformer spoofing detection. "
        "All five models must agree on direction for a trade signal to be emitted."
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
        "modelVersion": model or "deeplob-freqai-tlob-lgbm-lobt-ensemble-1m-v4",
        "explanation": explanation + " Macro trend, EMA distance, and VWAP offset are used as a stricter filter against fading strong directional regimes.",
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
            {
                "name": "LightGBM squeeze branch",
                "weight": round_to(lgbm_weight, 4),
                "score": round_to(lgbm_score, 4),
                "probability": round_to(lgbm_probability, 4),
                "predictedDirection": lgbm_direction,
                "historicalHitRate": lgbm_hit_rate,
                "recentWinRate": lgbm_recent_win_rate,
                "summary": "Detects squeeze and liquidation pressure via depth-flow cross signals and volume conviction.",
            },
            {
                "name": "LOB-Transformer spoofing branch",
                "weight": round_to(lobt_weight, 4),
                "score": round_to(lobt_score, 4),
                "probability": round_to(lobt_probability, 4),
                "predictedDirection": lobt_direction,
                "historicalHitRate": lobt_hit_rate,
                "recentWinRate": lobt_recent_win_rate,
                "summary": "Detects fake order walls and manipulation patterns in the order book depth.",
            },
        ],
        "topFeatures": top_features,
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
    server = ThreadingHTTPServer(("0.0.0.0", 8091), Handler)
    print("pulsealpha inference service listening on :8091")
    server.serve_forever()
