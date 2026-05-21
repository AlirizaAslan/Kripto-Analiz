package domain

import "time"

type UserRole string

const (
	RoleUser    UserRole = "user"
	RoleAdmin   UserRole = "admin"
	RoleAnalyst UserRole = "analyst"
)

type Market string

const (
	MarketUSEquities Market = "us_equities"
	MarketBIST       Market = "bist"
	MarketCrypto     Market = "crypto"
)

type SignalLabel string

const (
	SignalStrongBullish SignalLabel = "strong_bullish"
	SignalBullish       SignalLabel = "bullish"
	SignalNeutral       SignalLabel = "neutral"
	SignalBearish       SignalLabel = "bearish"
	SignalStrongBearish SignalLabel = "strong_bearish"
)

type RiskLabel string

const (
	RiskLow      RiskLabel = "low"
	RiskGuarded  RiskLabel = "guarded"
	RiskMedium   RiskLabel = "medium"
	RiskElevated RiskLabel = "elevated"
	RiskHigh     RiskLabel = "high"
)

type SignalQuality string

const (
	SignalQualityFullDepth SignalQuality = "full_depth"
	SignalQualityTopBook   SignalQuality = "top_of_book_only"
	SignalQualityDegraded  SignalQuality = "degraded"
)

type TradeAction string

const (
	TradeActionBuy     TradeAction = "buy"
	TradeActionSell    TradeAction = "sell"
	TradeActionHold    TradeAction = "hold"
	TradeActionNoTrade TradeAction = "no_trade"
)

type RegimeLabel string

const (
	RegimeTrend          RegimeLabel = "trend"
	RegimeRange          RegimeLabel = "range"
	RegimeHighVolatility RegimeLabel = "high_volatility"
	RegimeLowLiquidity   RegimeLabel = "low_liquidity"
)

type Asset struct {
	Symbol              string        `json:"symbol"`
	Name                string        `json:"name"`
	Market              Market        `json:"market"`
	MarketType          string        `json:"marketType"`
	InstrumentType      string        `json:"instrumentType"`
	Venue               string        `json:"venue"`
	SessionLabel        string        `json:"sessionLabel"`
	LastPrice           float64       `json:"lastPrice"`
	ChangePercent24H    float64       `json:"changePercent24h"`
	Volume              float64       `json:"volume"`
	Spread              float64       `json:"spread"`
	SignalLabel         SignalLabel   `json:"signalLabel"`
	ConfidenceScore     float64       `json:"confidenceScore"`
	VolatilityScore     float64       `json:"volatilityScore"`
	DepthImbalance      float64       `json:"depthImbalance"`
	OrderFlowImbalance  float64       `json:"orderFlowImbalance"`
	MicroPriceBias      float64       `json:"microPriceBias"`
	HistoricalHitRate   float64       `json:"historicalHitRate"`
	NextCandleInterval  string        `json:"nextCandleInterval"`
	RiskLabel           RiskLabel     `json:"riskLabel"`
	SignalQuality       SignalQuality `json:"signalQuality"`
	Thesis              string        `json:"thesis"`
	Range24H            PriceRange    `json:"range24h"`
	PrimaryExchangeCode string        `json:"primaryExchangeCode"`
}

type PriceRange struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

type DepthLevel struct {
	Price  float64 `json:"price"`
	Size   float64 `json:"size"`
	Orders int     `json:"orders"`
}

type DepthSnapshot struct {
	BestBid        float64       `json:"bestBid"`
	BestAsk        float64       `json:"bestAsk"`
	SpreadBps      float64       `json:"spreadBps"`
	DepthImbalance float64       `json:"depthImbalance"`
	MicroPrice     float64       `json:"microPrice"`
	Bids           []DepthLevel  `json:"bids"`
	Asks           []DepthLevel  `json:"asks"`
	LastUpdatedAt  time.Time     `json:"lastUpdatedAt"`
	SignalQuality  SignalQuality `json:"signalQuality"`
}

type FeatureAttribution struct {
	Name         string  `json:"name"`
	Value        float64 `json:"value"`
	Contribution float64 `json:"contribution"`
	Summary      string  `json:"summary"`
}

type ModelComponent struct {
	Name               string  `json:"name"`
	Weight             float64 `json:"weight"`
	Score              float64 `json:"score"`
	Probability        float64 `json:"probability"`
	PredictedDirection string  `json:"predictedDirection"`
	HistoricalHitRate  float64 `json:"historicalHitRate"`
	RecentWinRate      float64 `json:"recentWinRate"`
	Summary            string  `json:"summary"`
}

type Prediction struct {
	CandleInterval      string               `json:"candleInterval"`
	PredictionTimestamp time.Time            `json:"predictionTimestamp"`
	TargetCandleStart   time.Time            `json:"targetCandleStart"`
	PredictedDirection  string               `json:"predictedDirection"`
	UpProbability       float64              `json:"upProbability"`
	DownProbability     float64              `json:"downProbability"`
	NeutralProbability  float64              `json:"neutralProbability"`
	ConfidenceScore     float64              `json:"confidenceScore"`
	SignalLabel         SignalLabel          `json:"signalLabel"`
	RiskLabel           RiskLabel            `json:"riskLabel"`
	SignalQuality       SignalQuality        `json:"signalQuality"`
	HistoricalHitRate   float64              `json:"historicalHitRate"`
	ModelVersion        string               `json:"modelVersion"`
	ConsensusActive     bool                 `json:"consensusActive"`
	ConsensusDirection  string               `json:"consensusDirection"`
	ConsensusStrength   string               `json:"consensusStrength"`
	ConsensusSummary    string               `json:"consensusSummary"`
	TradeAllowed        bool                 `json:"tradeAllowed"`
	TradeAction         TradeAction          `json:"tradeAction"`
	TradeFilterReason   string               `json:"tradeFilterReason"`
	RegimeLabel         RegimeLabel          `json:"regimeLabel"`
	Explanation         string               `json:"explanation"`
	ModelComponents     []ModelComponent     `json:"modelComponents"`
	TopFeatures         []FeatureAttribution `json:"topFeatures"`
	Disclaimer          string               `json:"disclaimer"`
}

type PredictionHistoryItem struct {
	PredictedDirection string            `json:"predictedDirection"`
	RealizedDirection  string            `json:"realizedDirection"`
	TargetCandleStart  time.Time         `json:"targetCandleStart"`
	ConfidenceScore    float64           `json:"confidenceScore"`
	WasCorrect         bool              `json:"wasCorrect"`
	TradeAllowed       bool              `json:"tradeAllowed"`
	TradeAction        TradeAction       `json:"tradeAction"`
	ModelDirections    map[string]string `json:"modelDirections"`
	ConsensusActive    bool              `json:"consensusActive"`
	ConsensusDirection string            `json:"consensusDirection"`
	ConsensusStrength  string            `json:"consensusStrength"`
	TradeFilterReason  string            `json:"tradeFilterReason"`
	IsPending          bool              `json:"isPending"`
}

type Candle struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	IsLive    bool      `json:"isLive"`
}

type AccuracySummary struct {
	WinRate                  float64            `json:"winRate"`
	LifetimeWinRate          float64            `json:"lifetimeWinRate"`
	RecentWindowWinRate      float64            `json:"recentWindowWinRate"`
	Recent6WinRate           float64            `json:"recent6WinRate"`
	Recent7WinRate           float64            `json:"recent7WinRate"`
	ConsensusWinRate         float64            `json:"consensusWinRate"`
	TradeWinRate             float64            `json:"tradeWinRate"`
	TradeSampleSize          int                `json:"tradeSampleSize"`
	ConsensusTradeWinRate    float64            `json:"consensusTradeWinRate"`
	ConsensusTradeSampleSize int                `json:"consensusTradeSampleSize"`
	HorizonWinRates          map[string]float64 `json:"horizonWinRates"`
	HorizonSampleSizes       map[string]int     `json:"horizonSampleSizes"`
	CurrentStreak            int                `json:"currentStreak"`
	StreakDirection          string             `json:"streakDirection"`
	BullishAccuracy          float64            `json:"bullishAccuracy"`
	BearishAccuracy          float64            `json:"bearishAccuracy"`
	SampleSize               int                `json:"sampleSize"`
	LifetimeSampleSize       int                `json:"lifetimeSampleSize"`
	ConsensusSampleSize      int                `json:"consensusSampleSize"`
}

type AssetSummary struct {
	Asset            Asset           `json:"asset"`
	MiniCandles      []Candle        `json:"miniCandles"`
	Accuracy         AccuracySummary `json:"accuracy"`
	LatestPrediction Prediction      `json:"latestPrediction"`
}

type SymbolSnapshot struct {
	GeneratedAt       time.Time               `json:"generatedAt"`
	Asset             Asset                   `json:"asset"`
	Depth             DepthSnapshot           `json:"depth"`
	Prediction        Prediction              `json:"prediction"`
	Candles           []Candle                `json:"candles"`
	ChartCandles      map[string][]Candle     `json:"chartCandles"`
	PredictionHistory []PredictionHistoryItem `json:"predictionHistory"`
	Accuracy          AccuracySummary         `json:"accuracy"`
	WatchlistNote     string                  `json:"watchlistNote"`
	Disclaimer        string                  `json:"disclaimer"`
}

type MarketOverview struct {
	GeneratedAt  time.Time      `json:"generatedAt"`
	FeedStatus   string         `json:"feedStatus"`
	Disclaimer   string         `json:"disclaimer"`
	ActiveModels []string       `json:"activeModels"`
	Assets       []Asset        `json:"assets"`
	Summaries    []AssetSummary `json:"summaries"`
}

type HourlyAccuracy struct {
	Hour                    int     `json:"hour"`
	Total                   int     `json:"total"`
	Wins                    int     `json:"wins"`
	WinRate                 float64 `json:"winRate"`
	TradeTotal              int     `json:"tradeTotal"`
	TradeWins               int     `json:"tradeWins"`
	TradeWinRate            float64 `json:"tradeWinRate"`
	WrongCount              int     `json:"wrongCount"`
	WrongVolatilitySum      float64 `json:"wrongVolatilitySum"`
	AvgWrongVolatility      float64 `json:"avgWrongVolatility"`
	Pending                 int     `json:"pending"`
	AverageConfidence       float64 `json:"averageConfidence"`
	TradeAverageConfidence  float64 `json:"tradeAverageConfidence"`
	AveragePredictionVolume float64 `json:"averagePredictionVolume"`
	AverageResolvedVolume   float64 `json:"averageResolvedVolume"`
	PeakVolume              float64 `json:"peakVolume"`
}

type HourlyInsight struct {
	Label         string  `json:"label"`
	Hour          int     `json:"hour"`
	WinRate       float64 `json:"winRate"`
	TradeWinRate  float64 `json:"tradeWinRate"`
	SampleSize    int     `json:"sampleSize"`
	TradeSamples  int     `json:"tradeSamples"`
	AverageVolume float64 `json:"averageVolume"`
	PeakVolume    float64 `json:"peakVolume"`
}

type TradeFilterBreakdown struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type HourlyInsightSet struct {
	BestHours          []HourlyInsight `json:"bestHours"`
	WeakHours          []HourlyInsight `json:"weakHours"`
	MostActiveHours    []HourlyInsight `json:"mostActiveHours"`
	BestTradeHours     []HourlyInsight `json:"bestTradeHours"`
	HighestVolumeHours []HourlyInsight `json:"highestVolumeHours"`
	InactiveHours      []int           `json:"inactiveHours"`
	PendingHeavyHours  []int           `json:"pendingHeavyHours"`
}

type SymbolStatistics struct {
	Symbol                string                  `json:"symbol"`
	Market                Market                  `json:"market"`
	AccuracySummary       AccuracySummary         `json:"accuracySummary"`
	History               []PredictionHistoryItem `json:"history"`
	HourlyAccuracy        []HourlyAccuracy        `json:"hourlyAccuracy"`
	HourlyInsights        HourlyInsightSet        `json:"hourlyInsights"`
	TradeFilterBreakdown  []TradeFilterBreakdown  `json:"tradeFilterBreakdown"`
	PendingCount          int                     `json:"pendingCount"`
	TotalPredictions      int                     `json:"totalPredictions"`
	LastTargetCandleStart *time.Time              `json:"lastTargetCandleStart,omitempty"`
}

type StatisticsOverview struct {
	GeneratedAt time.Time          `json:"generatedAt"`
	Items       []SymbolStatistics `json:"items"`
}
