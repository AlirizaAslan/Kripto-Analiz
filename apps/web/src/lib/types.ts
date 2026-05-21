export type Market = 'us_equities' | 'bist' | 'crypto';
export type MarketType = 'stock' | 'crypto_spot' | 'crypto_perpetual';
export type SignalQuality = 'full_depth' | 'top_of_book_only' | 'degraded';
export type TradeAction = 'buy' | 'sell' | 'hold' | 'no_trade';
export type RegimeLabel = 'trend' | 'range' | 'high_volatility' | 'low_liquidity';

export type SignalLabel =
	| 'strong_bullish'
	| 'bullish'
	| 'neutral'
	| 'bearish'
	| 'strong_bearish';

export type RiskLabel = 'low' | 'guarded' | 'medium' | 'elevated' | 'high';
export type FeedMode = 'api_live' | 'ai_live' | 'heuristic_fallback';

export type Asset = {
	symbol: string;
	name: string;
	market: Market;
	marketType: MarketType;
	instrumentType: string;
	venue: string;
	sessionLabel: string;
	lastPrice: number;
	changePercent24h: number;
	volume: number;
	spread: number;
	signalLabel: SignalLabel;
	confidenceScore: number;
	volatilityScore: number;
	depthImbalance: number;
	orderFlowImbalance: number;
	microPriceBias: number;
	historicalHitRate: number;
	nextCandleInterval: string;
	riskLabel: RiskLabel;
	signalQuality: SignalQuality;
	thesis: string;
	range24h: {
		low: number;
		high: number;
	};
	primaryExchangeCode: string;
};

export type DepthLevel = {
	price: number;
	size: number;
	orders: number;
};

export type DepthSnapshot = {
	bestBid: number;
	bestAsk: number;
	spreadBps: number;
	depthImbalance: number;
	microPrice: number;
	bids: DepthLevel[];
	asks: DepthLevel[];
	lastUpdatedAt: string;
	signalQuality: SignalQuality;
};

export type FeatureAttribution = {
	name: string;
	value: number;
	contribution: number;
	summary: string;
};

export type ModelComponent = {
	name: string;
	weight: number;
	score: number;
	probability: number;
	predictedDirection: 'up' | 'down' | 'neutral';
	historicalHitRate: number;
	recentWinRate: number;
	summary: string;
};

export type Prediction = {
	candleInterval: string;
	predictionTimestamp: string;
	targetCandleStart: string;
	predictedDirection: 'up' | 'down' | 'neutral';
	upProbability: number;
	downProbability: number;
	neutralProbability: number;
	confidenceScore: number;
	signalLabel: SignalLabel;
	riskLabel: RiskLabel;
	signalQuality: SignalQuality;
	historicalHitRate: number;
	modelVersion: string;
	consensusActive: boolean;
	consensusDirection: 'up' | 'down' | 'neutral' | '';
	consensusStrength: string;
	consensusSummary: string;
	tradeAllowed: boolean;
	tradeAction: TradeAction;
	tradeFilterReason: string;
	regimeLabel: RegimeLabel;
	explanation: string;
	modelComponents: ModelComponent[];
	topFeatures: FeatureAttribution[];
	disclaimer: string;
};

export type PredictionHistoryItem = {
	predictedDirection: 'up' | 'down' | 'neutral';
	realizedDirection: 'up' | 'down' | 'neutral' | '';
	targetCandleStart: string;
	confidenceScore: number;
	wasCorrect: boolean;
	tradeAllowed: boolean;
	tradeAction: TradeAction;
	modelDirections: Record<string, 'up' | 'down' | 'neutral' | ''>;
	consensusActive: boolean;
	consensusDirection: 'up' | 'down' | 'neutral' | '';
	consensusStrength: string;
	tradeFilterReason: string;
	isPending: boolean;
};

export type Candle = {
	timestamp: string;
	open: number;
	high: number;
	low: number;
	close: number;
	volume: number;
	isLive: boolean;
};

export type AccuracySummary = {
	winRate: number;
	lifetimeWinRate: number;
	recentWindowWinRate: number;
	recent6WinRate: number;
	recent7WinRate: number;
	consensusWinRate: number;
	tradeWinRate: number;
	tradeSampleSize: number;
	consensusTradeWinRate: number;
	consensusTradeSampleSize: number;
	horizonWinRates: Record<string, number>;
	horizonSampleSizes: Record<string, number>;
	currentStreak: number;
	streakDirection: 'win' | 'loss' | '';
	bullishAccuracy: number;
	bearishAccuracy: number;
	sampleSize: number;
	lifetimeSampleSize: number;
	consensusSampleSize: number;
};

export type AssetSummary = {
	asset: Asset;
	miniCandles: Candle[];
	accuracy: AccuracySummary;
	latestPrediction: Prediction;
};

export type MarketOverview = {
	generatedAt: string;
	mode: FeedMode;
	disclaimer: string;
	activeModels: string[];
	feedStatus: string;
	assets: Asset[];
	summaries: AssetSummary[];
};

export type AssetDetail = {
	generatedAt: string;
	mode: FeedMode;
	asset: Asset;
	depth: DepthSnapshot;
	prediction: Prediction;
	candles: Candle[];
	chartCandles: Record<string, Candle[]>;
	predictionHistory: PredictionHistoryItem[];
	accuracy: AccuracySummary;
	watchlistNote: string;
	disclaimer: string;
};

export type HourlyAccuracy = {
	hour: number;
	total: number;
	wins: number;
	winRate: number;
	tradeTotal: number;
	tradeWins: number;
	tradeWinRate: number;
	wrongCount?: number;
	wrongVolatilitySum?: number;
	avgWrongVolatility?: number;
	pending: number;
	averageConfidence: number;
	tradeAverageConfidence: number;
	averagePredictionVolume: number;
	averageResolvedVolume: number;
	peakVolume: number;
};

export type HourlyInsight = {
	label: string;
	hour: number;
	winRate: number;
	tradeWinRate: number;
	sampleSize: number;
	tradeSamples: number;
	averageVolume: number;
	peakVolume: number;
};

export type HourlyInsightSet = {
	bestHours: HourlyInsight[];
	weakHours: HourlyInsight[];
	mostActiveHours: HourlyInsight[];
	bestTradeHours: HourlyInsight[];
	highestVolumeHours: HourlyInsight[];
	inactiveHours: number[];
	pendingHeavyHours: number[];
};

export type TradeFilterBreakdown = {
	reason: string;
	count: number;
};

export type SymbolStatistics = {
	symbol: string;
	market: Market;
	accuracySummary: AccuracySummary;
	history: PredictionHistoryItem[];
	hourlyAccuracy: HourlyAccuracy[];
	hourlyInsights: HourlyInsightSet;
	tradeFilterBreakdown: TradeFilterBreakdown[];
	pendingCount: number;
	totalPredictions: number;
	lastTargetCandleStart?: string;
};

export type StatisticsOverview = {
	generatedAt: string;
	items: SymbolStatistics[];
};
