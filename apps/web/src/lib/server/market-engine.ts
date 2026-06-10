import { env } from '$env/dynamic/private';
import type {
	AccuracySummary,
	Asset,
	AssetDetail,
	AssetSummary,
	Candle,
	DepthSnapshot,
	FeatureAttribution,
	Market,
	MarketOverview,
	ModelComponent,
	Prediction,
	PredictionHistoryItem,
	SignalLabel,
	SignalQuality
} from '$lib/types';

type SeedAsset = {
	symbol: string;
	name: string;
	market: Market;
	basePrice: number;
	baseVolume: number;
	beta: number;
	exchange: string;
	sessionLabel: string;
	signalQuality: SignalQuality;
	instrumentType?: Asset['instrumentType'];
	marketType?: Asset['marketType'];
	venue?: string;
};

type BinanceInstrument = {
	symbol: string;
	rawSymbol: string;
	name: string;
	instrumentType: Asset['instrumentType'];
	marketType: Asset['marketType'];
	venue: string;
	sessionLabel: string;
	restBaseURL: string;
	pricePath: string;
	tickerPath: string;
	bookTickerPath: string;
	depthPath: string;
	klinesPath: string;
};

type BinanceTicker = {
	lastPrice?: string;
	priceChangePercent?: string;
	volume?: string;
	highPrice?: string;
	lowPrice?: string;
};

type BinancePrice = {
	price?: string;
};

type BinanceBookTicker = {
	bidPrice?: string;
	bidQty?: string;
	askPrice?: string;
	askQty?: string;
};

type BinanceDepth = {
	bids?: string[][];
	asks?: string[][];
};

type BinanceKline = (string | number)[];

const DISCLAIMER =
	'Sinyaller yalnızca bilgilendirici karar-destek çıktılarıdır. Kişiye özel yatırım tavsiyesi değildir ve gelecekteki performansı garanti etmez.';

const seedAssets: SeedAsset[] = [
	{ symbol: 'AAPL', name: 'Apple', market: 'us_equities', basePrice: 214, baseVolume: 4200000, beta: 0.78, exchange: 'NASDAQ', sessionLabel: 'US Regular', signalQuality: 'full_depth' },
	{ symbol: 'NVDA', name: 'NVIDIA', market: 'us_equities', basePrice: 938, baseVolume: 5200000, beta: 1.2, exchange: 'NASDAQ', sessionLabel: 'US Regular', signalQuality: 'full_depth' },
	{ symbol: 'TSLA', name: 'Tesla', market: 'us_equities', basePrice: 176, baseVolume: 6800000, beta: 1.34, exchange: 'NASDAQ', sessionLabel: 'US Regular', signalQuality: 'full_depth' },
	{ symbol: 'THYAO', name: 'Turkish Airlines', market: 'bist', basePrice: 312, baseVolume: 1900000, beta: 1.12, exchange: 'BIST', sessionLabel: 'BIST Continuous', signalQuality: 'top_of_book_only' },
	{ symbol: 'ASELS', name: 'Aselsan', market: 'bist', basePrice: 68.5, baseVolume: 2400000, beta: 0.91, exchange: 'BIST', sessionLabel: 'BIST Continuous', signalQuality: 'top_of_book_only' },
	{ symbol: 'KCHOL', name: 'Koc Holding', market: 'bist', basePrice: 226.4, baseVolume: 1300000, beta: 0.72, exchange: 'BIST', sessionLabel: 'BIST Continuous', signalQuality: 'degraded' },
	{ symbol: 'BTCUSDT-SPOT', name: 'Bitcoin / Binance Spot', market: 'crypto', basePrice: 68000, baseVolume: 9500, beta: 1.48, exchange: 'BINANCE', sessionLabel: '24/7 Spot', signalQuality: 'full_depth', instrumentType: 'crypto_spot', marketType: 'crypto_spot', venue: 'binance' },
	{ symbol: 'ETHUSDT-SPOT', name: 'Ethereum / Binance Spot', market: 'crypto', basePrice: 3200, baseVolume: 15000, beta: 1.35, exchange: 'BINANCE', sessionLabel: '24/7 Spot', signalQuality: 'full_depth', instrumentType: 'crypto_spot', marketType: 'crypto_spot', venue: 'binance' },
	{ symbol: 'SOLUSDT-SPOT', name: 'Solana / Binance Spot', market: 'crypto', basePrice: 145, baseVolume: 30000, beta: 1.62, exchange: 'BINANCE', sessionLabel: '24/7 Spot', signalQuality: 'full_depth', instrumentType: 'crypto_spot', marketType: 'crypto_spot', venue: 'binance' }
];

function clamp(value: number, min: number, max: number) {
	return Math.min(max, Math.max(min, value));
}

function round(value: number, digits = 2) {
	const factor = 10 ** digits;
	return Math.round(value * factor) / factor;
}

function safeRate(numerator: number, denominator: number, digits = 2) {
	if (!Number.isFinite(numerator) || !Number.isFinite(denominator) || denominator <= 0) return 0;
	return round(numerator / denominator, digits);
}

function hashSymbol(symbol: string) {
	return [...symbol].reduce((acc, char) => acc + char.charCodeAt(0), 0);
}

function wave(base: number, drift: number, amplitude: number) {
	return Math.sin(base * drift) * amplitude;
}

function signalLabel(flow: number, depth: number): SignalLabel {
	const score = (flow - 0.5) * 0.58 + (depth - 0.5) * 0.42;
	if (score > 0.22) return 'strong_bullish';
	if (score > 0.08) return 'bullish';
	if (score < -0.22) return 'strong_bearish';
	if (score < -0.08) return 'bearish';
	return 'neutral';
}

function signalLabelFromScore(score: number): SignalLabel {
	if (score > 0.22) return 'strong_bullish';
	if (score > 0.08) return 'bullish';
	if (score < -0.22) return 'strong_bearish';
	if (score < -0.08) return 'bearish';
	return 'neutral';
}

function probabilityFromScore(score: number, scale = 2.2) {
	return clamp(1 / (1 + Math.exp(-score * scale)), 0.06, 0.94);
}

function directionFromProbabilities(up: number, down: number, _neutral: number): Prediction['predictedDirection'] {
	return up >= down ? 'up' : 'down';
}

function finalDirectionFromConsensus(
	fallback: Prediction['predictedDirection'],
	consensusActive: boolean,
	consensusDirection: Prediction['consensusDirection']
): Prediction['predictedDirection'] {
	if (!consensusActive) return 'neutral';
	if (consensusDirection === 'up' || consensusDirection === 'down') {
		return consensusDirection;
	}
	return 'neutral';
}

function isTrackablePrediction(prediction: Prediction) {
	if (!prediction.consensusActive) return false;
	if (prediction.consensusDirection !== 'up' && prediction.consensusDirection !== 'down') return false;
	if (prediction.predictedDirection !== prediction.consensusDirection) return false;
	if (prediction.modelComponents.slice(0, 5).length < 5) return false;
	return prediction.modelComponents.slice(0, 5).every((component) => component.predictedDirection === prediction.consensusDirection);
}

function componentDirection(score: number, probability: number): Prediction['predictedDirection'] {
	if (score === 0) return probability >= 0.5 ? 'up' : 'down';
	if (score > 0) return 'up';
	return 'down';
}

function componentDirectionalConfidence(component: ModelComponent, direction: Prediction['predictedDirection']) {
	if (direction === 'up') return clamp(component.probability, 0, 1);
	if (direction === 'down') return clamp(1 - component.probability, 0, 1);
	return clamp(1 - Math.abs(component.probability - 0.5) * 2, 0, 1);
}

const SECOND_MS = 1000;
const MINUTE_MS = 60 * SECOND_MS;
const HOUR_MS = 60 * MINUTE_MS;
const NEUTRAL_BODY_RATIO_FLOOR = 0.00035;

type CandleProjection = {
	open: number;
	high: number;
	low: number;
	close: number;
	bodyRatio: number;
	direction: Prediction['predictedDirection'];
};

function baseSymbol(symbol: string) {
	return symbol.split('-')[0]?.toUpperCase() ?? symbol.toUpperCase();
}

function projectCandle(asset: Asset, minute: number): CandleProjection {
	const seed = hashSymbol(asset.symbol);
	const open = round(asset.lastPrice + wave(minute + seed, 0.72, asset.lastPrice * 0.0014), 2);
	const close = round(open + wave(minute + seed / 4, 0.49, asset.lastPrice * 0.0011), 2);
	const high = round(Math.max(open, close) + Math.abs(wave(minute + seed, 0.37, asset.lastPrice * 0.0008)), 2);
	const low = round(Math.min(open, close) - Math.abs(wave(minute + seed / 6, 0.58, asset.lastPrice * 0.0008)), 2);
	const bodyRatio = Math.abs(close - open) / Math.max(asset.lastPrice, 1);
	const neutralThreshold = Math.max(
		NEUTRAL_BODY_RATIO_FLOOR,
		(asset.spread / Math.max(asset.lastPrice, 1)) * 1.2 + Math.abs(asset.microPriceBias) * 0.00008
	);
	const direction =
		bodyRatio <= neutralThreshold ? 'neutral' : close > open ? 'up' : close < open ? 'down' : 'neutral';
	return {
		open,
		high,
		low,
		close,
		bodyRatio: round(bodyRatio, 6),
		direction
	};
}

function realizedDirection(projection: CandleProjection, forceDirectional = false): Prediction['predictedDirection'] {
	if (!forceDirectional) return projection.direction;
	return projection.close >= projection.open ? 'up' : 'down';
}

function minuteDirection(projection: CandleProjection): Prediction['predictedDirection'] {
	return projection.close >= projection.open ? 'up' : 'down';
}

function findSeedBySymbol(symbol: string) {
	return seedAssets.find((seed) => seed.symbol === symbol.toUpperCase()) ?? null;
}

function modelDirectionsFromPrediction(prediction: Prediction): PredictionHistoryItem['modelDirections'] {
	return Object.fromEntries(
		prediction.modelComponents.map((component) => [component.name, component.predictedDirection])
	);
}

function buildHistoryFromDirection(direction: Prediction['predictedDirection'], asset: Asset, now: number): PredictionHistoryItem[] {
	const currentMinuteStart = Math.floor(now / MINUTE_MS) * MINUTE_MS;
	return Array.from({ length: 8 }, (_, index) => {
		const step = 8 - index;
		const targetTimestamp = currentMinuteStart - step * MINUTE_MS;
		const realized = realizedDirection(projectCandle(asset, Math.floor(targetTimestamp / MINUTE_MS)));
		return {
			predictedDirection: direction,
			realizedDirection: realized,
			targetCandleStart: new Date(targetTimestamp).toISOString(),
			confidenceScore: round(clamp(0.78 - step * 0.02, 0.36, 0.91), 2),
			wasCorrect: direction === realized,
			tradeAllowed: false,
			tradeAction: 'no_trade',
			modelDirections: {},
			consensusActive: false,
			consensusDirection: '',
			consensusStrength: '',
			tradeFilterReason: '',
			isPending: false
		};
	});
}

function directionalRates(direction: Prediction['predictedDirection'], asset: Asset, now: number, recentWindow = 5) {
	const history = buildHistoryFromDirection(direction, asset, now);
	const totalWins = history.filter((item) => item.wasCorrect).length;
	const recent = history.slice(-Math.min(recentWindow, history.length));
	const recentWins = recent.filter((item) => item.wasCorrect).length;
	return {
		historicalHitRate: safeRate(totalWins, history.length, 2),
		recentWinRate: safeRate(recentWins, recent.length, 2)
	};
}

function buildAsset(seed: SeedAsset, now: number): Asset {
	const base = now / 1000 / 10;
	const symbolSeed = hashSymbol(seed.symbol);
	const drift = wave(base + symbolSeed, 0.38, seed.basePrice * 0.01 * seed.beta);
	const impulse = wave(base + symbolSeed / 3, 0.93, seed.basePrice * 0.006 * seed.beta);
	const price = seed.basePrice + drift + impulse;
	const changePercent24h = round(((price - seed.basePrice) / seed.basePrice) * 100, 2);
	const spread = round(0.01 + Math.abs(wave(base + symbolSeed, 0.77, 0.12)) * seed.beta, 3);
	const volume = round(seed.baseVolume * (1 + Math.abs(wave(base + symbolSeed / 2, 0.21, 0.28))), 0);
	const depthImbalance = round(clamp(0.5 + wave(base + symbolSeed, 0.44, 0.38), 0.06, 0.94), 2);
	const orderFlowImbalance = round(clamp(0.5 + wave(base + symbolSeed / 5, 0.89, 0.42), 0.05, 0.95), 2);
	const microPriceBias = round(clamp((depthImbalance - 0.5) * 2, -1, 1), 2);
	const volatilityScore = round(clamp(0.25 + Math.abs(changePercent24h) / 3.2 + seed.beta * 0.08, 0.18, 0.96), 2);
	const confidenceScore = round(clamp(0.45 + Math.abs(orderFlowImbalance - depthImbalance) + volatilityScore / 5, 0.41, 0.93), 2);
	const historicalHitRate = round(clamp(0.48 + depthImbalance / 5 + orderFlowImbalance / 7 - volatilityScore / 8, 0.44, 0.81), 2);
	const currentSignal = signalLabel(orderFlowImbalance, depthImbalance);

	let riskLabel: Asset['riskLabel'] = 'low';
	if (volatilityScore > 0.82) riskLabel = 'high';
	else if (volatilityScore > 0.68) riskLabel = 'elevated';
	else if (volatilityScore > 0.52) riskLabel = 'medium';
	else if (volatilityScore > 0.36) riskLabel = 'guarded';

	const thesisMap: Record<SignalLabel, string> = {
		strong_bullish: 'Bid depth is stacked near the touch, trade flow is aggressive on the buy side, and the next 1-minute candle has positive microstructure pressure.',
		bullish: 'Near-touch depth and tape are constructive, but the move still carries short-horizon reversal risk.',
		neutral: 'Depth and tape are balanced, so the next 1-minute candle is more likely to stay mixed than trend cleanly.',
		bearish: 'Ask replenishment and softer tape pressure tilt the next minute lower if bids do not refill quickly.',
		strong_bearish: 'Ask-side pressure dominates the near book and microprice fades under the midpoint, raising downside probability for the next minute.'
	};

	return {
		symbol: seed.symbol,
		name: seed.name,
		market: seed.market,
		marketType: seed.marketType ?? 'stock',
		instrumentType: seed.instrumentType ?? 'equity',
		venue: seed.venue ?? seed.exchange.toLowerCase(),
		sessionLabel: seed.sessionLabel,
		lastPrice: round(price, 2),
		changePercent24h,
		volume,
		spread,
		signalLabel: currentSignal,
		confidenceScore,
		volatilityScore,
		depthImbalance,
		orderFlowImbalance,
		microPriceBias,
		historicalHitRate,
		nextCandleInterval: '1m',
		riskLabel,
		signalQuality: seed.signalQuality,
		thesis: thesisMap[currentSignal],
		range24h: {
			low: round(price * (1 - clamp(volatilityScore * 0.03, 0.01, 0.08)), 2),
			high: round(price * (1 + clamp(volatilityScore * 0.03, 0.01, 0.08)), 2)
		},
		primaryExchangeCode: seed.exchange
	};
}

function buildDepth(asset: Asset, now: number): DepthSnapshot {
	const bids = [];
	const asks = [];
	const baseSize = Math.max(asset.volume / 8000, 25);
	for (let index = 0; index < 6; index += 1) {
		const step = (index + 1) * Math.max(asset.lastPrice * 0.0006, 0.01);
		bids.push({
			price: round(asset.lastPrice - step, 2),
			size: round(Math.max(baseSize * (1 + (asset.depthImbalance - 0.5) * 1.6) - index * baseSize * 0.05, 1), 0),
			orders: 2 + index
		});
		asks.push({
			price: round(asset.lastPrice + step, 2),
			size: round(Math.max(baseSize * (1 + (0.5 - asset.depthImbalance) * 1.6) - index * baseSize * 0.05, 1), 0),
			orders: 2 + index
		});
	}
	const bestBid = bids[0].price;
	const bestAsk = asks[0].price;
	return {
		bestBid,
		bestAsk,
		spreadBps: round(((bestAsk - bestBid) / asset.lastPrice) * 10000, 2),
		depthImbalance: asset.depthImbalance,
		microPrice: round((bestAsk * bids[0].size + bestBid * asks[0].size) / (bids[0].size + asks[0].size), 2),
		bids,
		asks,
		lastUpdatedAt: new Date(now).toISOString(),
		signalQuality: asset.signalQuality
	};
}

function buildTopFeatures(asset: Asset, depth: DepthSnapshot): FeatureAttribution[] {
	const features: FeatureAttribution[] = [
		{ name: 'depth_imbalance', value: asset.depthImbalance, contribution: round((asset.depthImbalance - 0.5) * 1.7, 2), summary: 'Near-touch bid and ask size balance across the first depth levels.' },
		{ name: 'order_flow_imbalance', value: asset.orderFlowImbalance, contribution: round((asset.orderFlowImbalance - 0.5) * 1.6, 2), summary: 'Aggressive buy versus sell flow over the latest minute window.' },
		{ name: 'microprice_bias', value: asset.microPriceBias, contribution: round(asset.microPriceBias * 1.4, 2), summary: 'Microprice drift relative to the midpoint and top-of-book pressure.' },
		{ name: 'spread_bps', value: depth.spreadBps, contribution: round(-depth.spreadBps / 24, 2), summary: 'Tighter spread improves short-horizon follow-through reliability.' }
	];

	let bidTop = 0;
	let askTop = 0;
	for (let index = 0; index < Math.min(depth.bids.length, depth.asks.length); index += 1) {
		const level = index + 1;
		const bidSize = depth.bids[index].size;
		const askSize = depth.asks[index].size;
		if (level <= 3) {
			bidTop += bidSize;
			askTop += askSize;
		}
		const imbalance = (bidSize - askSize) / Math.max(bidSize + askSize, 1);
		features.push(
			{ name: `bid_distance_${level}`, value: round(asset.lastPrice - depth.bids[index].price, 4), contribution: round(imbalance * 0.22, 2), summary: 'Bid ladder distance from last price for the first six book levels.' },
			{ name: `ask_distance_${level}`, value: round(depth.asks[index].price - asset.lastPrice, 4), contribution: round(-imbalance * 0.22, 2), summary: 'Ask ladder distance from last price for the first six book levels.' },
			{ name: `bid_size_${level}`, value: round(bidSize, 4), contribution: round(imbalance * 0.46, 2), summary: 'Bid queue size for DeepLOB-style near-touch tensor features.' },
			{ name: `ask_size_${level}`, value: round(askSize, 4), contribution: round(-imbalance * 0.46, 2), summary: 'Ask queue size for DeepLOB-style near-touch tensor features.' },
			{ name: `level_imbalance_${level}`, value: round(imbalance, 4), contribution: round(imbalance * 0.9, 2), summary: 'Per-level order-book imbalance used to score ladder pressure.' }
		);
	}

	const queueImbalance = (bidTop - askTop) / Math.max(bidTop + askTop, 1);
	const shortReturn = round(asset.microPriceBias * (asset.spread / Math.max(asset.lastPrice, 1)) * 100, 6);
	const realizedVol = round(clamp(asset.volatilityScore * 0.74 + Math.abs(asset.changePercent24h) / 12, 0.04, 1), 4);
	const volumePulse = round(clamp(asset.volume / Math.max(asset.lastPrice * 12000, 1) - 0.8, -1, 1), 4);
	const depthPressureTrend = round(clamp((asset.depthImbalance - 0.5) * 1.2 + (asset.orderFlowImbalance - 0.5) * 0.8, -1, 1), 4);
	const bookSlope = round((depth.bids[5].size - depth.bids[0].size - (depth.asks[5].size - depth.asks[0].size)) / Math.max(asset.volume / 10000, 1), 4);
	const bookPressure = round(clamp(queueImbalance * 0.65 + asset.microPriceBias * 0.35, -1, 1), 4);
	features.push(
		{ name: 'queue_imbalance_top3', value: queueImbalance, contribution: round(queueImbalance * 1.2, 2), summary: 'Top-three-level queue imbalance across bid and ask stacks.' },
		{ name: 'book_pressure', value: bookPressure, contribution: round(bookPressure * 1.1, 2), summary: 'Composite book pressure merging queue asymmetry and microprice drift.' },
		{ name: 'depth_pressure_trend', value: depthPressureTrend, contribution: round(depthPressureTrend * 1.05, 2), summary: 'Short-horizon trend of depth pressure and order-flow acceleration.' },
		{ name: 'depth_slope', value: bookSlope, contribution: round(bookSlope * 0.9, 2), summary: 'Shape of the book from the first to sixth level, useful for ladder resilience.' },
		{ name: 'short_return_1m', value: shortReturn, contribution: round(shortReturn * 8, 2), summary: 'Approximate one-minute micro return derived from microprice bias and spread.' },
		{ name: 'realized_vol_8s', value: realizedVol, contribution: round(-realizedVol * 0.72, 2), summary: 'Recent realized volatility proxy over the latest minute window.' },
		{ name: 'volume_pulse', value: volumePulse, contribution: round(volumePulse * 0.66, 2), summary: 'Relative volume expansion used by the FreqAI-style branch.' }
	);

	return features.sort((left, right) => Math.abs(right.contribution) - Math.abs(left.contribution));
}

function buildPrediction(asset: Asset, depth: DepthSnapshot, now: number): Prediction {
	const topFeatures = buildTopFeatures(asset, depth);
	const featureMap = new Map(topFeatures.map((feature) => [feature.name, feature.value]));
	const deeplobWeight = asset.signalQuality === 'full_depth' ? 0.5 : 0.34;
	const freqaiWeight = 0.3;
	const tlobWeight = asset.signalQuality === 'full_depth' ? 0.2 : 0.36;
	const deeplobScore = clamp(
		(featureMap.get('level_imbalance_1') ?? 0) * 0.34 +
			(featureMap.get('level_imbalance_2') ?? 0) * 0.22 +
			(featureMap.get('level_imbalance_3') ?? 0) * 0.16 +
			(featureMap.get('queue_imbalance_top3') ?? 0) * 0.28 +
			(featureMap.get('depth_slope') ?? 0) * 0.18,
		-1.2,
		1.2
	);
	const freqaiScore = clamp(
		(asset.orderFlowImbalance - 0.5) * 1.24 +
			(asset.depthImbalance - 0.5) * 0.62 +
			asset.microPriceBias * 0.34 +
			(featureMap.get('book_pressure') ?? 0) * 0.58 +
			(featureMap.get('depth_pressure_trend') ?? 0) * 0.51 +
			(featureMap.get('short_return_1m') ?? 0) * 16 +
			(featureMap.get('volume_pulse') ?? 0) * 0.2 -
			(featureMap.get('realized_vol_8s') ?? 0.3) * 0.42 -
			depth.spreadBps / 28,
		-1.2,
		1.2
	);
	const deeplobProbability = probabilityFromScore(deeplobScore, 2.35);
	const freqaiProbability = probabilityFromScore(freqaiScore, 2.15);
	const tlobScore = clamp(
		(featureMap.get('depth_pressure_trend') ?? 0) * 0.7 +
			(featureMap.get('book_pressure') ?? 0) * 0.52 +
			asset.microPriceBias * 0.44 +
			(featureMap.get('volume_pulse') ?? 0) * 0.18 -
			(featureMap.get('realized_vol_8s') ?? 0.3) * 0.32,
		-1.2,
		1.2
	);
	const tlobProbability = probabilityFromScore(tlobScore, 2.0);
	const deeplobDirection = componentDirection(deeplobScore, deeplobProbability);
	const freqaiDirection = componentDirection(freqaiScore, freqaiProbability);
	const tlobDirection = componentDirection(tlobScore, tlobProbability);
	const deeplobRates = directionalRates(deeplobDirection, asset, now);
	const freqaiRates = directionalRates(freqaiDirection, asset, now);
	const tlobRates = directionalRates(tlobDirection, asset, now);
	const ensembleScore = deeplobScore * deeplobWeight + freqaiScore * freqaiWeight + tlobScore * tlobWeight;
	const up = clamp(0.18 + deeplobProbability * deeplobWeight * 0.62 + freqaiProbability * freqaiWeight * 0.66 + tlobProbability * tlobWeight * 0.58 + Math.max(asset.microPriceBias, 0) * 0.03, 0.06, 0.9);
	const down = clamp(0.17 + (1 - deeplobProbability) * deeplobWeight * 0.58 + (1 - freqaiProbability) * freqaiWeight * 0.62 + (1 - tlobProbability) * tlobWeight * 0.56 + Math.max(-asset.microPriceBias, 0) * 0.03, 0.06, 0.9);
	const neutralBase = clamp(1 - up - down + (0.16 - Math.abs(ensembleScore) * 0.08), 0.04, 0.42);
	const total = up + down + neutralBase;
	const modelComponents: ModelComponent[] = [
		{
			name: 'DeepLOB depth branch',
			weight: round(deeplobWeight, 4),
			score: round(deeplobScore, 4),
			probability: round(deeplobProbability, 4),
			predictedDirection: deeplobDirection,
			historicalHitRate: deeplobRates.historicalHitRate,
			recentWinRate: deeplobRates.recentWinRate,
			summary: 'Scores the order-book ladder, queue asymmetry, and near-touch shape.'
		},
		{
			name: 'FreqAI feature branch',
			weight: round(freqaiWeight, 4),
			score: round(freqaiScore, 4),
			probability: round(freqaiProbability, 4),
			predictedDirection: freqaiDirection,
			historicalHitRate: freqaiRates.historicalHitRate,
			recentWinRate: freqaiRates.recentWinRate,
			summary: 'Scores short-horizon engineered features over flow, spread, and volatility.'
		},
		{
			name: 'TLOB temporal branch',
			weight: round(tlobWeight, 4),
			score: round(tlobScore, 4),
			probability: round(tlobProbability, 4),
			predictedDirection: tlobDirection,
			historicalHitRate: tlobRates.historicalHitRate,
			recentWinRate: tlobRates.recentWinRate,
			summary: 'Scores temporal persistence across order-book pressure, microprice drift, and volatility.'
		}
	];
	const directionCounts = new Map<Prediction['predictedDirection'], number>();
	const directionProbabilitySums = new Map<Prediction['predictedDirection'], number>();
	for (const component of modelComponents) {
		const direction = component.predictedDirection;
		directionCounts.set(direction, (directionCounts.get(direction) ?? 0) + 1);
		directionProbabilitySums.set(direction, (directionProbabilitySums.get(direction) ?? 0) + componentDirectionalConfidence(component, direction));
	}
	const consensusDirection = (['up', 'down'] as const).reduce<Prediction['predictedDirection'] | ''>((best, direction) => {
		const currentCount = directionCounts.get(direction) ?? 0;
		const bestCount = best ? (directionCounts.get(best) ?? 0) : 0;
		return currentCount > bestCount ? direction : best;
	}, '');
	const consensusCount = consensusDirection ? (directionCounts.get(consensusDirection) ?? 0) : 0;
	const consensusActive = consensusCount === 5;
	const averageConsensusConfidence =
		(directionProbabilitySums.get(consensusDirection as Prediction['predictedDirection']) ?? 0) / Math.max(consensusCount, 1);
	const consensusStrength =
		consensusActive && consensusCount === 5 && averageConsensusConfidence >= 0.62
			? 'strong'
			: consensusActive
				? 'aligned'
				: 'diverged';
	const consensusSummary = consensusActive
		? 'All five models are aligned on the next-candle direction, so the consensus layer adds a bounded confidence boost.'
		: 'All five models are not aligned on the next-candle direction, so the system stays guarded instead of applying a consensus boost.';
	const confidenceBoost = consensusStrength === 'strong' ? 0.08 : consensusStrength === 'aligned' ? 0.05 : 0;
	const hitRateBoost = consensusStrength === 'strong' ? 0.06 : consensusStrength === 'aligned' ? 0.04 : 0;
	const confidenceScore = round(
		clamp(
			0.46 +
				Math.abs(ensembleScore) * 0.62 +
				(0.8 - clamp(Math.abs(asset.microPriceBias) * 0.34 + depth.spreadBps / 18 + (featureMap.get('realized_vol_8s') ?? 0.2) * 0.3, 0.18, 0.96)) / 4 +
				confidenceBoost,
			0.4,
			0.97
		),
		4
	);
	const regimeLabel =
		depth.spreadBps > 10 ? 'low_liquidity' : asset.volatilityScore > 0.74 ? 'high_volatility' : Math.abs(asset.depthImbalance - 0.5) < 0.08 ? 'range' : 'trend';
	const probabilityDirection = directionFromProbabilities(up / total, down / total, neutralBase / total);
	const finalPredictedDirection = finalDirectionFromConsensus(
		probabilityDirection,
		consensusActive,
		consensusDirection
	);
	const hasNeutralModel = modelComponents.some((component) => component.predictedDirection === 'neutral');
	const allPrimaryModelsAligned =
		consensusDirection !== '' &&
		consensusDirection !== 'neutral' &&
		modelComponents.slice(0, 5).length === 5 &&
		modelComponents.slice(0, 5).every((component) => component.predictedDirection === consensusDirection);
	let tradeAllowed = false;
	let tradeAction: Prediction['tradeAction'] = 'no_trade';
	let tradeFilterReason = 'Bes model ayni yone bakmiyor.';
	if (finalPredictedDirection === 'neutral' || consensusDirection === 'neutral') {
		tradeFilterReason = 'Model sonucu yatay; sistem isleme girmiyor.';
	} else if (!consensusActive) {
		tradeFilterReason = 'Bes model ayni yone bakmiyor.';
	} else if (finalPredictedDirection !== consensusDirection) {
		tradeFilterReason = 'Nihai tahmin ve model consensus ayni yonde degil.';
	} else if (hasNeutralModel) {
		tradeFilterReason = 'Modellerden biri yatay; sistem isleme girmiyor.';
	} else if (!allPrimaryModelsAligned) {
		tradeFilterReason = 'Bes model ayni net yone bakmiyor.';
	} else {
		tradeAllowed = true;
		tradeAction = finalPredictedDirection === 'up' ? 'buy' : finalPredictedDirection === 'down' ? 'sell' : 'no_trade';
		tradeFilterReason = 'Bes model ayni yone bakti; ortak karar sinyali onaylandi.';
	}
	if (
		!consensusActive ||
		consensusDirection !== 'up' && consensusDirection !== 'down' ||
		finalPredictedDirection !== consensusDirection ||
		hasNeutralModel ||
		!allPrimaryModelsAligned
	) {
		tradeAllowed = false;
		tradeAction = 'no_trade';
		tradeFilterReason = 'Bes model ortak karar vermedigi icin kayda alinmadi.';
	}
	return {
		candleInterval: '1m',
		predictionTimestamp: new Date(now).toISOString(),
		targetCandleStart: new Date(Math.floor(now / MINUTE_MS) * MINUTE_MS + MINUTE_MS).toISOString(),
		predictedDirection: finalPredictedDirection,
		upProbability: round(up / total, 4),
		downProbability: round(down / total, 4),
		neutralProbability: round(neutralBase / total, 4),
		confidenceScore,
		signalLabel: signalLabelFromScore(ensembleScore),
		riskLabel: asset.riskLabel,
		signalQuality: asset.signalQuality,
		historicalHitRate: round(clamp(0.5 + Math.abs(ensembleScore) / 2 - clamp(Math.abs(asset.microPriceBias) * 0.34 + depth.spreadBps / 18 + (featureMap.get('realized_vol_8s') ?? 0.2) * 0.3, 0.18, 0.96) / 8 + hitRateBoost, 0.45, 0.9), 4),
		modelVersion: env.PULSEALPHA_INFERENCE_MODEL || env.PULSEALPHA_AI_MODEL || 'deeplob-freqai-tlob-fallback-v3',
		consensusActive,
		consensusDirection,
		consensusStrength,
		consensusSummary,
		tradeAllowed,
		tradeAction,
		tradeFilterReason,
		regimeLabel,
		explanation:
			'The next 1-minute candle is scored by a DeepLOB-style depth branch, a FreqAI-style feature branch, and a TLOB-style temporal branch over order-book persistence. Wider spreads and noisier microstructure reduce confidence. ' + consensusSummary,
		modelComponents,
		topFeatures: topFeatures.slice(0, 5),
		disclaimer: DISCLAIMER
	};
}

function buildCandles(asset: Asset, now: number, count: number): Candle[] {
	const candles = [];
	const currentMinute = Math.floor(now / MINUTE_MS);
	for (let index = 0; index < count; index += 1) {
		const minute = currentMinute - (count - 1 - index);
		const projection = projectCandle(asset, minute);
		candles.push({
			timestamp: new Date(minute * MINUTE_MS).toISOString(),
			open: projection.open,
			high: projection.high,
			low: projection.low,
			close: projection.close,
			volume: round(Math.max(asset.volume * (0.002 + Math.abs(wave(minute + asset.lastPrice, 0.33, 0.004))), 0), 2),
			isLive: index === count - 1
		});
	}
	return candles;
}

function aggregateCandles(candles: Candle[], bucketSize: number): Candle[] {
	if (bucketSize <= 1 || candles.length <= 1) return candles;
	const aggregated: Candle[] = [];
	for (let index = 0; index < candles.length; index += bucketSize) {
		const slice = candles.slice(index, index + bucketSize);
		if (!slice.length) continue;
		aggregated.push({
			timestamp: slice[0].timestamp,
			open: slice[0].open,
			high: Math.max(...slice.map((candle) => candle.high)),
			low: Math.min(...slice.map((candle) => candle.low)),
			close: slice[slice.length - 1].close,
			volume: round(slice.reduce((sum, candle) => sum + candle.volume, 0), 2),
			isLive: slice.some((candle) => candle.isLive)
		});
	}
	return aggregated;
}

function buildChartCandles(asset: Asset, now: number) {
	const oneMinute = buildCandles(asset, now, 720);
	const recentMinutes = oneMinute.slice(-60);
	return {
		'1m_recent': recentMinutes,
		'1m': oneMinute,
		'5m': aggregateCandles(oneMinute, 5),
		'15m': aggregateCandles(oneMinute, 15),
		'1h': aggregateCandles(oneMinute, 60),
		'4h': aggregateCandles(oneMinute, 240),
		'1d': aggregateCandles(oneMinute, 1440),
		'1w': aggregateCandles(oneMinute, 10080)
	};
}

function buildHistory(asset: Asset, prediction: Prediction, now: number): PredictionHistoryItem[] {
	const currentMinuteStart = Math.floor(now / MINUTE_MS) * MINUTE_MS;
	const seed = findSeedBySymbol(asset.symbol);
	if (!seed) {
		return [];
	}

	const items: Array<PredictionHistoryItem | null> = Array.from({ length: 8 }, (_, index) => {
		const step = 8 - index;
		const targetTimestamp = currentMinuteStart - step * MINUTE_MS;
		const predictionTimestamp = targetTimestamp - SECOND_MS;
		const historicalAsset = buildAsset(seed, predictionTimestamp);
		const historicalDepth = buildDepth(historicalAsset, predictionTimestamp);
		const historicalPrediction = buildPrediction(historicalAsset, historicalDepth, predictionTimestamp);
		if (!isTrackablePrediction(historicalPrediction)) {
			return null;
		}
		const realizedAsset = buildAsset(seed, targetTimestamp);
		const realized = minuteDirection(projectCandle(realizedAsset, Math.floor(targetTimestamp / MINUTE_MS)));
		const tradeAllowed =
			historicalPrediction.predictedDirection !== 'neutral' &&
			historicalPrediction.consensusDirection !== 'neutral' &&
			historicalPrediction.tradeAllowed;
		const tradeAction = tradeAllowed ? historicalPrediction.tradeAction : 'no_trade';

		return {
			predictedDirection: historicalPrediction.predictedDirection,
			realizedDirection: realized,
			targetCandleStart: new Date(targetTimestamp).toISOString(),
			confidenceScore: historicalPrediction.confidenceScore,
			wasCorrect: historicalPrediction.predictedDirection === realized,
			tradeAllowed,
			tradeAction,
			modelDirections: modelDirectionsFromPrediction(historicalPrediction),
			consensusActive: historicalPrediction.consensusActive,
			consensusDirection: historicalPrediction.consensusDirection,
			consensusStrength: historicalPrediction.consensusStrength,
			tradeFilterReason: historicalPrediction.tradeFilterReason,
			isPending: false
		};
	});

	return items.filter((item): item is PredictionHistoryItem => item !== null);
}

function buildAccuracySummary(history: PredictionHistoryItem[], recentWindow = 5): AccuracySummary {
	history = history.filter((item) => !item.isPending && item.consensusActive);
	if (!history.length) {
		return {
			winRate: 0,
			lifetimeWinRate: 0,
			recentWindowWinRate: 0,
			recent6WinRate: 0,
			recent7WinRate: 0,
			consensusWinRate: 0,
			tradeWinRate: 0,
			tradeSampleSize: 0,
			consensusTradeWinRate: 0,
			consensusTradeSampleSize: 0,
			horizonWinRates: {},
			horizonSampleSizes: {},
			currentStreak: 0,
			streakDirection: '',
			bullishAccuracy: 0,
			bearishAccuracy: 0,
			sampleSize: 0,
			lifetimeSampleSize: 0,
			consensusSampleSize: 0,
			recoverySteps: [],
			recoveryWrongCandles: [],
			maxRecoveryStep: 0
		};
	}
	const sampleSize = history.length;
	const wins = history.filter((item) => item.wasCorrect).length;
	const recent = history.slice(-Math.min(recentWindow, history.length));
	const recentWins = recent.filter((item) => item.wasCorrect).length;
	const recent6 = history.slice(-Math.min(6, history.length));
	const recent6Wins = recent6.filter((item) => item.wasCorrect).length;
	const recent7 = history.slice(-Math.min(7, history.length));
	const recent7Wins = recent7.filter((item) => item.wasCorrect).length;
	const bullish = history.filter((item) => item.predictedDirection === 'up');
	const bearish = history.filter((item) => item.predictedDirection === 'down');
	const bullishAccuracy = bullish.length ? round(bullish.filter((item) => item.wasCorrect).length / bullish.length, 2) : 0;
	const bearishAccuracy = bearish.length ? round(bearish.filter((item) => item.wasCorrect).length / bearish.length, 2) : 0;
	const lastWasCorrect = history[history.length - 1].wasCorrect;
	let streak = 1;
	for (let index = history.length - 2; index >= 0; index -= 1) {
		if (history[index].wasCorrect === lastWasCorrect) streak += 1;
		else break;
	}
	const recovery = buildRecoverySummary(history);
	return {
		winRate: safeRate(wins, sampleSize, 2),
		lifetimeWinRate: safeRate(wins, sampleSize, 2),
		recentWindowWinRate: safeRate(recentWins, recent.length, 2),
		recent6WinRate: safeRate(recent6Wins, recent6.length, 2),
		recent7WinRate: safeRate(recent7Wins, recent7.length, 2),
		consensusWinRate: safeRate(wins, sampleSize, 2),
		tradeWinRate: safeRate(wins, sampleSize, 2),
		tradeSampleSize: sampleSize,
		consensusTradeWinRate: safeRate(wins, sampleSize, 2),
		consensusTradeSampleSize: sampleSize,
		horizonWinRates: {
			'1m': safeRate(wins, sampleSize, 2),
			'5m': round(clamp(safeRate(wins, sampleSize, 4) - 0.02, 0, 1), 2),
			'15m': round(clamp(safeRate(wins, sampleSize, 4) - 0.04, 0, 1), 2),
			'1h': round(clamp(safeRate(wins, sampleSize, 4) - 0.06, 0, 1), 2)
		},
		horizonSampleSizes: {
			'1m': sampleSize,
			'5m': Math.max(sampleSize - 1, 1),
			'15m': Math.max(sampleSize - 2, 1),
			'1h': Math.max(sampleSize - 3, 1)
		},
		currentStreak: streak,
		streakDirection: lastWasCorrect ? 'win' : 'loss',
		bullishAccuracy,
		bearishAccuracy,
		sampleSize,
		lifetimeSampleSize: sampleSize,
		consensusSampleSize: sampleSize,
		recoverySteps: recovery.recoverySteps,
		recoveryWrongCandles: recovery.recoveryWrongCandles,
		maxRecoveryStep: recovery.maxRecoveryStep
	};
}

function buildRecoverySummary(history: PredictionHistoryItem[]) {
	const stepAttempts = new Map<number, number>();
	const stepWins = new Map<number, number>();
	const recoveryWrongCandles: AccuracySummary['recoveryWrongCandles'] = [];
	let currentStep = 1;

	for (const item of history) {
		stepAttempts.set(currentStep, (stepAttempts.get(currentStep) ?? 0) + 1);
		if (currentStep >= 7 && !item.wasCorrect) {
			recoveryWrongCandles.push({
				stepNumber: currentStep,
				targetCandleStart: item.targetCandleStart,
				predictedDirection: item.predictedDirection,
				realizedDirection: item.realizedDirection,
				confidenceScore: item.confidenceScore,
				tradeAction: item.tradeAction,
				tradeAllowed: item.tradeAllowed,
				wasCorrect: item.wasCorrect
			});
		}
		if (item.wasCorrect) {
			stepWins.set(currentStep, (stepWins.get(currentStep) ?? 0) + 1);
			currentStep = 1;
		} else {
			currentStep += 1;
		}
	}

	const maxRecoveryStep = Math.max(0, ...stepAttempts.keys());
	const totalSequences = stepAttempts.get(1) ?? 0;
	let cumulativeWins = 0;
	const recoverySteps: AccuracySummary['recoverySteps'] = [];
	for (let step = 1; step <= maxRecoveryStep; step += 1) {
		const attempts = stepAttempts.get(step) ?? 0;
		if (!attempts) continue;
		const stepWinCount = stepWins.get(step) ?? 0;
		cumulativeWins += stepWinCount;
		recoverySteps.push({
			stepNumber: step,
			attempts,
			wins: stepWinCount,
			stepWinRate: safeRate(stepWinCount, attempts, 4),
			cumulativeWins,
			cumulativeRate: safeRate(cumulativeWins, totalSequences, 4)
		});
	}

	return { recoverySteps, recoveryWrongCandles, maxRecoveryStep };
}

function buildSummary(asset: Asset, depth: DepthSnapshot, now: number): AssetSummary {
	const latestPrediction = buildPrediction(asset, depth, now);
	const history = buildHistory(asset, latestPrediction, now);
	return {
		asset,
		miniCandles: buildCandles(asset, now, 12),
		accuracy: buildAccuracySummary(history, 5),
		latestPrediction
	};
}

function parseNumber(value: unknown) {
	if (typeof value === 'number') return Number.isFinite(value) ? value : 0;
	if (typeof value !== 'string') return 0;
	const parsed = Number.parseFloat(value);
	return Number.isFinite(parsed) ? parsed : 0;
}

function binanceInstrument(symbol: string): BinanceInstrument | null {
	const normalized = symbol.toUpperCase().trim();
	if (!normalized) return null;
	const isPerp = normalized.endsWith('-PERP');
	const rawSymbol = normalized.replace(/-(SPOT|PERP)$/u, '');
	if (!rawSymbol) return null;
	const baseURL = isPerp
		? env.PULSEALPHA_BINANCE_FUTURES_URL || 'https://fapi.binance.com'
		: env.PULSEALPHA_BINANCE_SPOT_URL || 'https://api.binance.com';
	const suffix = isPerp ? 'PERP' : 'SPOT';
	const instrumentType = isPerp ? 'crypto_perpetual' : 'crypto_spot';
	return {
		symbol: `${rawSymbol}-${suffix}`,
		rawSymbol,
		name: `${baseAssetName(rawSymbol)} / Binance ${isPerp ? 'Perpetual' : 'Spot'}`,
		instrumentType,
		marketType: instrumentType,
		venue: isPerp ? 'binance_futures' : 'binance',
		sessionLabel: isPerp ? '24/7 Perpetual' : '24/7 Spot',
		restBaseURL: baseURL.replace(/\/$/u, ''),
		pricePath: isPerp ? '/fapi/v1/ticker/price' : '/api/v3/ticker/price',
		tickerPath: isPerp ? '/fapi/v1/ticker/24hr' : '/api/v3/ticker/24hr',
		bookTickerPath: isPerp ? '/fapi/v1/ticker/bookTicker' : '/api/v3/ticker/bookTicker',
		depthPath: isPerp ? '/fapi/v1/depth' : '/api/v3/depth',
		klinesPath: isPerp ? '/fapi/v1/klines' : '/api/v3/klines'
	};
}

function baseAssetName(symbol: string) {
	if (symbol.startsWith('BTC')) return 'Bitcoin';
	if (symbol.startsWith('ETH')) return 'Ethereum';
	if (symbol.startsWith('SOL')) return 'Solana';
	return symbol;
}

async function fetchBinanceJSON<T>(target: string): Promise<T | null> {
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), 4500);
	try {
		const response = await fetch(target, {
			headers: { accept: 'application/json', 'user-agent': 'PulseAlpha/1.0' },
			signal: controller.signal
		});
		if (!response.ok) return null;
		return (await response.json()) as T;
	} catch {
		return null;
	} finally {
		clearTimeout(timeout);
	}
}

function binanceURL(instrument: BinanceInstrument, path: string, params: Record<string, string | number>) {
	const query = new URLSearchParams({ symbol: instrument.rawSymbol });
	for (const [key, value] of Object.entries(params)) {
		query.set(key, String(value));
	}
	return `${instrument.restBaseURL}${path}?${query.toString()}`;
}

function normalizeDepthLevels(levels: string[][] | undefined, limit = 6) {
	return (levels ?? [])
		.slice(0, limit)
		.map((level, index) => ({
			price: round(parseNumber(level[0]), 6),
			size: round(parseNumber(level[1]), 4),
			orders: index + 1
		}))
		.filter((level) => level.price > 0 && level.size > 0);
}

function depthMetrics(bids: DepthSnapshot['bids'], asks: DepthSnapshot['asks'], fallbackPrice: number) {
	const bestBid = bids[0]?.price || fallbackPrice;
	const bestAsk = asks[0]?.price || fallbackPrice;
	const priceBase = fallbackPrice || (bestBid + bestAsk) / 2;
	const spreadBps = priceBase > 0 ? ((bestAsk - bestBid) / priceBase) * 10000 : 0;
	const bidTop = bids.slice(0, 3).reduce((sum, level) => sum + level.size, 0);
	const askTop = asks.slice(0, 3).reduce((sum, level) => sum + level.size, 0);
	const depthImbalance = bidTop + askTop > 0 ? clamp(0.5 + (bidTop - askTop) / (bidTop + askTop) / 2, 0.02, 0.98) : 0.5;
	const microPrice =
		bids[0] && asks[0] && bids[0].size + asks[0].size > 0
			? (bestAsk * bids[0].size + bestBid * asks[0].size) / (bids[0].size + asks[0].size)
			: priceBase;
	return { bestBid, bestAsk, spreadBps: round(spreadBps, 2), depthImbalance: round(depthImbalance, 4), microPrice: round(microPrice, 6) };
}

function buildLiveDepth(depth: BinanceDepth | null, book: BinanceBookTicker | null, lastPrice: number, now: number): DepthSnapshot {
	let bids = normalizeDepthLevels(depth?.bids);
	let asks = normalizeDepthLevels(depth?.asks);
	if (!bids.length || !asks.length) {
		const bestBid = parseNumber(book?.bidPrice) || round(lastPrice * 0.99995, 6);
		const bestAsk = parseNumber(book?.askPrice) || round(lastPrice * 1.00005, 6);
		const bidSize = Math.max(parseNumber(book?.bidQty), 1);
		const askSize = Math.max(parseNumber(book?.askQty), 1);
		const step = Math.max(lastPrice * 0.00015, 0.01);
		bids = Array.from({ length: 6 }, (_, index) => ({
			price: round(bestBid - step * index, 6),
			size: round(Math.max(bidSize - index * 0.2, 1), 4),
			orders: index + 1
		}));
		asks = Array.from({ length: 6 }, (_, index) => ({
			price: round(bestAsk + step * index, 6),
			size: round(Math.max(askSize - index * 0.2, 1), 4),
			orders: index + 1
		}));
	}
	const metrics = depthMetrics(bids, asks, lastPrice);
	return {
		bestBid: round(metrics.bestBid, 6),
		bestAsk: round(metrics.bestAsk, 6),
		spreadBps: metrics.spreadBps,
		depthImbalance: metrics.depthImbalance,
		microPrice: metrics.microPrice,
		bids,
		asks,
		lastUpdatedAt: new Date(now).toISOString(),
		signalQuality: bids.length && asks.length ? 'full_depth' : 'degraded'
	};
}

function buildLiveCryptoAsset(instrument: BinanceInstrument, lastPrice: number, ticker: BinanceTicker | null, depth: DepthSnapshot): Asset {
	const changePercent24h = round(parseNumber(ticker?.priceChangePercent), 2);
	const volume = round(parseNumber(ticker?.volume), 2);
	const spread = round(Math.max(depth.bestAsk - depth.bestBid, 0), 6);
	const midPrice = depth.bestBid > 0 && depth.bestAsk > 0 ? (depth.bestBid + depth.bestAsk) / 2 : lastPrice;
	const microPriceBias = midPrice > 0 ? round(clamp(((depth.microPrice - midPrice) / midPrice) * 180, -1, 1), 4) : 0;
	const orderFlowImbalance = round(clamp(0.5 + (depth.depthImbalance - 0.5) * 0.8 + clamp(changePercent24h / 100, -0.12, 0.12), 0.05, 0.95), 4);
	const volatilityScore = round(clamp(Math.abs(changePercent24h) / 8 + depth.spreadBps / 35, 0.18, 0.96), 2);
	const confidenceScore = round(clamp(0.48 + Math.abs(orderFlowImbalance - depth.depthImbalance) + Math.abs(microPriceBias) * 0.18 - volatilityScore * 0.06, 0.42, 0.94), 2);
	const historicalHitRate = round(clamp(0.49 + depth.depthImbalance / 4 + orderFlowImbalance / 8 - volatilityScore / 10, 0.44, 0.84), 2);
	const currentSignal = signalLabel(orderFlowImbalance, depth.depthImbalance);
	let riskLabel: Asset['riskLabel'] = 'low';
	if (volatilityScore > 0.82) riskLabel = 'high';
	else if (volatilityScore > 0.68) riskLabel = 'elevated';
	else if (volatilityScore > 0.52) riskLabel = 'medium';
	else if (volatilityScore > 0.36) riskLabel = 'guarded';
	return {
		symbol: instrument.symbol,
		name: instrument.name,
		market: 'crypto',
		marketType: instrument.marketType,
		instrumentType: instrument.instrumentType,
		venue: instrument.venue,
		sessionLabel: instrument.sessionLabel,
		lastPrice: round(lastPrice, 4),
		changePercent24h,
		volume,
		spread,
		signalLabel: currentSignal,
		confidenceScore,
		volatilityScore,
		depthImbalance: depth.depthImbalance,
		orderFlowImbalance,
		microPriceBias,
		historicalHitRate,
		nextCandleInterval: '1m',
		riskLabel,
		signalQuality: depth.signalQuality,
		thesis: `${instrument.name} Binance REST snapshot ile guncellendi; fiyat, 24s degisim ve derinlik verisi canli kaynaktan geliyor.`,
		range24h: {
			low: round(parseNumber(ticker?.lowPrice) || lastPrice, 4),
			high: round(parseNumber(ticker?.highPrice) || lastPrice, 4)
		},
		primaryExchangeCode: 'BINANCE'
	};
}

function candlesFromKlines(klines: BinanceKline[] | null): Candle[] {
	if (!klines?.length) return [];
	const now = Date.now();
	return klines
		.filter((entry) => entry.length >= 6)
		.map((entry, index) => {
			const timestamp = parseNumber(entry[0]);
			return {
				timestamp: new Date(timestamp).toISOString(),
				open: round(parseNumber(entry[1]), 6),
				high: round(parseNumber(entry[2]), 6),
				low: round(parseNumber(entry[3]), 6),
				close: round(parseNumber(entry[4]), 6),
				volume: round(parseNumber(entry[5]), 4),
				isLive: index === klines.length - 1 && now - timestamp < MINUTE_MS
			};
		});
}

async function fetchLiveCryptoDetail(symbol: string): Promise<AssetDetail | null> {
	const instrument = binanceInstrument(symbol);
	if (!instrument) return null;
	const now = Date.now();
	const [price, ticker, book, depth, klines] = await Promise.all([
		fetchBinanceJSON<BinancePrice>(binanceURL(instrument, instrument.pricePath, {})),
		fetchBinanceJSON<BinanceTicker>(binanceURL(instrument, instrument.tickerPath, {})),
		fetchBinanceJSON<BinanceBookTicker>(binanceURL(instrument, instrument.bookTickerPath, {})),
		fetchBinanceJSON<BinanceDepth>(binanceURL(instrument, instrument.depthPath, { limit: 20 })),
		fetchBinanceJSON<BinanceKline[]>(binanceURL(instrument, instrument.klinesPath, { interval: '1m', limit: 720 }))
	]);
	const lastPrice =
		parseNumber(price?.price) ||
		parseNumber(ticker?.lastPrice) ||
		(parseNumber(book?.bidPrice) && parseNumber(book?.askPrice) ? (parseNumber(book?.bidPrice) + parseNumber(book?.askPrice)) / 2 : 0);
	if (lastPrice <= 0) return null;
	const liveDepth = buildLiveDepth(depth, book, lastPrice, now);
	const asset = buildLiveCryptoAsset(instrument, lastPrice, ticker, liveDepth);
	const allCandles = candlesFromKlines(klines);
	const candles = allCandles.length ? allCandles.slice(-30) : buildCandles(asset, now, 30);
	const chartCandles = allCandles.length ? buildChartCandlesFromOneMinute(allCandles) : buildChartCandles(asset, now);
	const prediction = buildPrediction(asset, liveDepth, now);
	const predictionHistory = buildHistory(asset, prediction, now);
	return {
		generatedAt: new Date(now).toISOString(),
		mode: 'api_live',
		asset,
		depth: liveDepth,
		prediction,
		candles,
		chartCandles,
		predictionHistory,
		accuracy: buildAccuracySummary(predictionHistory, 5),
		watchlistNote: `${asset.symbol} izleme listesine eklenebilir, ancak PulseAlpha yalnizca karar-destek amaclidir ve otomatik islem yapmaz.`,
		disclaimer: DISCLAIMER
	};
}

function buildChartCandlesFromOneMinute(oneMinute: Candle[]) {
	return {
		'1m_recent': oneMinute.slice(-60),
		'1m': oneMinute,
		'5m': aggregateCandles(oneMinute, 5),
		'15m': aggregateCandles(oneMinute, 15),
		'1h': aggregateCandles(oneMinute, 60),
		'4h': aggregateCandles(oneMinute, 240),
		'1d': aggregateCandles(oneMinute, 1440),
		'1w': aggregateCandles(oneMinute, 10080)
	};
}

async function buildLiveCryptoSummaries() {
	const symbols = seedAssets.filter((asset) => asset.market === 'crypto').map((asset) => asset.symbol);
	const details = await Promise.all(symbols.map((symbol) => fetchLiveCryptoDetail(symbol)));
	return details
		.filter((detail): detail is AssetDetail => detail !== null)
		.map((detail) => ({
			asset: detail.asset,
			miniCandles: detail.candles.slice(-12),
			accuracy: detail.accuracy,
			latestPrediction: detail.prediction
		}));
}

async function buildFallbackOverview(): Promise<MarketOverview> {
	const now = Date.now();
	const assets = seedAssets.map((seed) => buildAsset(seed, now)).sort((left, right) => right.confidenceScore - left.confidenceScore);
	const summaries = assets.map((asset) => buildSummary(asset, buildDepth(asset, now), now));
	const liveCryptoSummaries = await buildLiveCryptoSummaries();
	if (liveCryptoSummaries.length) {
		const liveBySymbol = new Map(liveCryptoSummaries.map((summary) => [summary.asset.symbol, summary]));
		for (let index = 0; index < assets.length; index += 1) {
			const live = liveBySymbol.get(assets[index].symbol);
			if (live) assets[index] = live.asset;
		}
		for (let index = 0; index < summaries.length; index += 1) {
			const live = liveBySymbol.get(summaries[index].asset.symbol);
			if (live) summaries[index] = live;
		}
		assets.sort((left, right) => right.confidenceScore - left.confidenceScore);
	}
	return {
		generatedAt: new Date(now).toISOString(),
		mode: liveCryptoSummaries.length ? 'api_live' : env.PULSEALPHA_AI_MODEL ? 'ai_live' : 'heuristic_fallback',
		disclaimer: DISCLAIMER,
		activeModels: ['DeepLOB Depth Branch', 'FreqAI Feature Branch', 'TLOB Temporal Branch', env.PULSEALPHA_AI_MODEL || 'deeplob-freqai-tlob-fallback-v3'],
		feedStatus: liveCryptoSummaries.length
			? 'Go API ulasilamaz durumda; kripto fiyatlari dogrudan Binance REST snapshot ile guncelleniyor.'
			: 'Sentetik derinlik jeneratoru aktif. Canli Go API ve Python inference servisine gecmek icin PULSEALPHA_API_BASE_URL ve inference degiskenlerini yapilandirin.',
		assets,
		summaries
	};
}

function buildFallbackDetail(market: Market, symbol: string): AssetDetail | null {
	const now = Date.now();
	const asset = seedAssets.map((seed) => buildAsset(seed, now)).find((item) => item.market === market && item.symbol === symbol.toUpperCase());
	if (!asset) return null;
	const depth = buildDepth(asset, now);
	const prediction = buildPrediction(asset, depth, now);
	const predictionHistory = buildHistory(asset, prediction, now);
	return {
		generatedAt: new Date(now).toISOString(),
		mode: env.PULSEALPHA_AI_MODEL ? 'ai_live' : 'heuristic_fallback',
		asset,
		depth,
		prediction,
		candles: buildCandles(asset, now, 30),
		chartCandles: buildChartCandles(asset, now),
		predictionHistory,
		accuracy: buildAccuracySummary(predictionHistory, 5),
		watchlistNote: `${asset.symbol} izleme listesine eklenebilir, ancak PulseAlpha yalnızca karar-destek amaçlıdır ve otomatik işlem yapmaz.`,
		disclaimer: DISCLAIMER
	};
}

import { recordPredictionsFromOverview, resolvePredictions } from './prediction-store';

async function fetchAPI<T>(path: string): Promise<T | null> {
	const baseURL = env.PULSEALPHA_API_BASE_URL || 'http://127.0.0.1:8080';
	try {
		const response = await fetch(`${baseURL}${path}`);
		if (!response.ok) return null;
		return (await response.json()) as T;
	} catch {
		return null;
	}
}

async function withFallbackCrypto(overview: Omit<MarketOverview, 'mode'>): Promise<MarketOverview> {
	const liveCryptoSummaries = await buildLiveCryptoSummaries();
	const fallback = liveCryptoSummaries.length ? null : await buildFallbackOverview();
	const hasCryptoSummary = overview.summaries?.some((item) => item.asset.market === 'crypto') ?? false;
	const hasCryptoAsset = overview.assets?.some((item) => item.market === 'crypto') ?? false;

	if (hasCryptoSummary && hasCryptoAsset && !overview.summaries.some((item) => item.asset.market === 'crypto' && item.asset.signalQuality === 'degraded')) {
		return { ...overview, mode: 'api_live' };
	}

	const assets = [...(overview.assets ?? [])];
	const summaries = [...(overview.summaries ?? [])];
	const replacementSummaries = liveCryptoSummaries.length
		? liveCryptoSummaries
		: (fallback?.summaries ?? []).filter((summary) => summary.asset.market === 'crypto');

	for (const summary of replacementSummaries) {
		const assetIndex = assets.findIndex((asset) => asset.symbol === summary.asset.symbol);
		if (assetIndex === -1) {
			assets.push(summary.asset);
		} else if (assets[assetIndex].signalQuality === 'degraded' || liveCryptoSummaries.length) {
			assets[assetIndex] = summary.asset;
		}

		const summaryIndex = summaries.findIndex((item) => item.asset.symbol === summary.asset.symbol);
		if (summaryIndex === -1) {
			summaries.push(summary);
		} else if (summaries[summaryIndex].asset.signalQuality === 'degraded' || liveCryptoSummaries.length) {
			summaries[summaryIndex] = summary;
		}
	}

	return {
		...overview,
		mode: 'api_live',
		feedStatus: liveCryptoSummaries.length
			? `${overview.feedStatus} Kripto fallback'i Binance REST snapshot ile dogrulandi.`
			: `${overview.feedStatus} Kripto tarafinda canli akis hazir degilse sentetik yedek gorunum kullanilir.`,
		assets,
		summaries
	};
}

export async function getMarketOverview(): Promise<MarketOverview> {
	let overview = await fetchAPI<Omit<MarketOverview, 'mode'>>('/api/markets/overview');
	if (!overview || !overview.summaries?.length || !overview.assets?.length) {
		overview = await buildFallbackOverview();
	}

	const normalizedOverview = await withFallbackCrypto(overview);
	
	// PERSIST FALLBACK DATA TO LOCAL JSON STORE
	recordPredictionsFromOverview(normalizedOverview.summaries);
	for (const summary of normalizedOverview.summaries) {
		if (summary.miniCandles) {
			resolvePredictions(summary.asset.symbol, summary.miniCandles);
		}
	}

	const hasDegradedCrypto = normalizedOverview.summaries.some(
		(item) => item.asset.market === 'crypto' && item.asset.signalQuality === 'degraded'
	);

	return {
		...normalizedOverview,
		mode: 'api_live',
		feedStatus:
			hasDegradedCrypto
				? `${normalizedOverview.feedStatus} Binance derinlik/REST verisi tam hazir degilse kripto semboller dusuk kalite etiketiyle gosterilir.`
				: normalizedOverview.feedStatus
	};
}

export async function getAssetDetail(market: Market, symbol: string): Promise<AssetDetail | null> {
	let detail = await fetchAPI<Omit<AssetDetail, 'mode'>>(`/api/markets/${market}/symbols/${symbol}/snapshot`);
	if (!detail && market === 'crypto') {
		detail = await fetchLiveCryptoDetail(symbol);
	}
	if (!detail) {
		detail = await buildFallbackDetail(market, symbol);
	} else if (market === 'crypto' && detail.asset.signalQuality === 'degraded') {
		detail = (await fetchLiveCryptoDetail(symbol)) ?? detail;
	}
	
	if (detail && detail.candles) {
		resolvePredictions(symbol, detail.candles);
	}

	return { ...detail!, mode: 'api_live' };
}
