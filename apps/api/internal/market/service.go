package market

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pulsealpha/api/internal/config"
	"pulsealpha/api/internal/domain"
	"pulsealpha/api/internal/inference"
	"pulsealpha/api/internal/marketdata"
	"pulsealpha/api/internal/storage"
)

const disclaimer = "Sinyaller yalnızca bilgilendirici karar-destek çıktılarıdır. Kişiye özel yatırım tavsiyesi değildir ve gelecekteki performansı garanti etmez."

const predictionCandleInterval = time.Minute

type seedAsset struct {
	Symbol       string
	Name         string
	Market       domain.Market
	BasePrice    float64
	BaseVolume   float64
	Beta         float64
	ExchangeCode string
	SessionLabel string
	Quality      domain.SignalQuality
}

type cachedOverview struct {
	data      domain.MarketOverview
	expiresAt time.Time
}

type cachedSnapshot struct {
	data      domain.SymbolSnapshot
	expiresAt time.Time
}

type Service struct {
	inference       *inference.Client
	provider        *marketdata.BinanceProvider
	store           *storage.PredictionStore
	filter          tradeFilterConfig
	mu              sync.Mutex
	trackers        map[string]*predictionTracker
	predictionCache map[string]cachedPrediction

	overviewMu    sync.Mutex
	overviewCache cachedOverview

	snapshotMu    sync.Mutex
	snapshotLocks map[string]*sync.Mutex
	snapshotCache map[string]cachedSnapshot
}

type tradeFilterConfig struct {
	MinConfidence       float64
	MaxSpreadBps        float64
	MinConsensusSamples int
	MinDepthImbalance   float64
	MinMicroPriceBias   float64
}

func NewService(client *inference.Client, provider *marketdata.BinanceProvider, store *storage.PredictionStore, cfg config.Config) *Service {
	return &Service{
		inference: client,
		provider:  provider,
		store:     store,
		filter: tradeFilterConfig{
			MinConfidence:       cfg.MinConfidence,
			MaxSpreadBps:        cfg.MaxSpreadBps,
			MinConsensusSamples: cfg.MinConsensusSamples,
			MinDepthImbalance:   cfg.MinDepthImbalance,
			MinMicroPriceBias:   cfg.MinMicroPriceBias,
		},
		trackers:        make(map[string]*predictionTracker),
		predictionCache: make(map[string]cachedPrediction),
		snapshotLocks:   make(map[string]*sync.Mutex),
		snapshotCache:   make(map[string]cachedSnapshot),
	}
}

func (s *Service) Overview() domain.MarketOverview {
	s.overviewMu.Lock()
	defer s.overviewMu.Unlock()

	if time.Now().Before(s.overviewCache.expiresAt) {
		return s.overviewCache.data
	}

	now := time.Now().UTC()
	assets := make([]domain.Asset, 0, 10)
	summaries := make([]domain.AssetSummary, 0, 10)

	if s.provider != nil && s.provider.Enabled() {
		for _, instrument := range s.provider.Instruments() {
			snapshot, err := s.provider.Snapshot(instrument.Symbol)
			if err != nil {
				continue
			}
			asset, depth, prediction := s.buildCryptoSnapshot(snapshot, now)
			candles := trimCandles(snapshot.Candles, 60)
			if len(candles) == 0 {
				continue
			}
			prediction, _, accuracy := s.applyTrackedAccuracy(asset, depth, prediction, candles)
			miniCandles := trimCandles(candles, 12)
			assets = append(assets, asset)
			summaries = append(summaries, domain.AssetSummary{
				Asset:            asset,
				MiniCandles:      miniCandles,
				Accuracy:         accuracy,
				LatestPrediction: prediction,
			})
		}
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].ConfidenceScore > assets[j].ConfidenceScore
	})

	status := "Kripto semboller için canlı Binance derinlik verisi bekleniyor."
	if len(assets) > 0 {
		if s.inference != nil && s.inference.Enabled() {
			status = "Kripto semboller için canlı Binance derinlik verisi ve yapay zeka servisi aktif."
		} else {
			status = "Kripto semboller için canlı Binance derinlik verisi aktif, yapay zeka fallback devrede."
		}
	}

	result := domain.MarketOverview{
		GeneratedAt: now,
		FeedStatus:  status,
		Disclaimer:  disclaimer,
		ActiveModels: []string{
			"DeepLOB Depth Branch",
			"FreqAI Feature Branch",
			"TLOB Temporal Branch",
			s.inferenceModel(),
		},
		Assets:    assets,
		Summaries: summaries,
	}

	s.overviewCache = cachedOverview{
		data:      result,
		expiresAt: time.Now().Add(4 * time.Second),
	}

	return result
}

func (s *Service) getSnapshotLock(symbol string) *sync.Mutex {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	lock, ok := s.snapshotLocks[symbol]
	if !ok {
		lock = &sync.Mutex{}
		s.snapshotLocks[symbol] = lock
	}
	return lock
}

func (s *Service) Snapshot(market domain.Market, symbol string) (domain.SymbolSnapshot, error) {
	if market != domain.MarketCrypto {
		return domain.SymbolSnapshot{}, errors.New("unsupported market")
	}
	if s.provider == nil || !s.provider.Enabled() {
		return domain.SymbolSnapshot{}, errors.New("crypto provider not configured")
	}

	lock := s.getSnapshotLock(symbol)
	lock.Lock()
	defer lock.Unlock()

	if cache, ok := s.snapshotCache[symbol]; ok && time.Now().Before(cache.expiresAt) {
		return cache.data, nil
	}

	now := time.Now().UTC()
	live, err := s.provider.Snapshot(symbol)
	if err != nil {
		return domain.SymbolSnapshot{}, err
	}

	asset, depth, prediction := s.buildCryptoSnapshot(live, now)
	candles := trimCandles(live.Candles, 30)
	if len(candles) == 0 {
		return domain.SymbolSnapshot{}, errors.New("no live candles available")
	}

	prediction, history, accuracy := s.applyTrackedAccuracy(asset, depth, prediction, candles)

	result := domain.SymbolSnapshot{
		GeneratedAt:       now,
		Asset:             asset,
		Depth:             depth,
		Prediction:        prediction,
		Candles:           candles,
		ChartCandles:      ensureChartCandles(live.ChartCandles, candles),
		PredictionHistory: history,
		Accuracy:          accuracy,
		WatchlistNote:     asset.Symbol + " izleme listesine eklenebilir, ancak sistem yalnızca karar-destek amaçlıdır ve otomatik işlem yapmaz.",
		Disclaimer:        disclaimer,
	}

	s.snapshotCache[symbol] = cachedSnapshot{
		data:      result,
		expiresAt: time.Now().Add(4 * time.Second),
	}

	return result, nil
}

func (s *Service) buildCryptoSnapshot(snapshot marketdata.Snapshot, now time.Time) (domain.Asset, domain.DepthSnapshot, domain.Prediction) {
	asset := buildCryptoAsset(snapshot)
	features := buildFeatures(asset, snapshot.Depth)
	prediction := s.buildPrediction(asset, snapshot.Depth, features, trimCandles(snapshot.Candles, 30), now)
	return asset, snapshot.Depth, prediction
}

func (s *Service) findCryptoInstrument(symbol string) (marketdata.Instrument, bool) {
	if s.provider == nil {
		return marketdata.Instrument{}, false
	}
	needle := strings.ToUpper(strings.TrimSpace(symbol))
	for _, instrument := range s.provider.Instruments() {
		if instrument.Symbol == needle {
			return instrument, true
		}
	}
	return marketdata.Instrument{}, false
}

func (s *Service) buildCryptoPlaceholderSnapshot(instrument marketdata.Instrument, now time.Time) (domain.Asset, domain.DepthSnapshot, domain.Prediction, []domain.Candle) {
	seed := seedAsset{
		Symbol:       instrument.Symbol,
		Name:         instrument.Name,
		Market:       domain.MarketCrypto,
		BasePrice:    cryptoBasePrice(instrument.RawSymbol),
		BaseVolume:   cryptoBaseVolume(instrument.RawSymbol),
		Beta:         cryptoBeta(instrument.RawSymbol),
		ExchangeCode: instrument.ExchangeCode,
		SessionLabel: instrument.SessionLabel,
		Quality:      domain.SignalQualityDegraded,
	}

	asset := buildAsset(seed, now)
	asset.Name = instrument.Name
	asset.Market = domain.MarketCrypto
	asset.MarketType = instrument.InstrumentType
	asset.InstrumentType = instrument.InstrumentType
	asset.Venue = instrument.Venue
	asset.SessionLabel = instrument.SessionLabel
	asset.PrimaryExchangeCode = instrument.ExchangeCode
	asset.SignalQuality = domain.SignalQualityDegraded
	depth := buildDepth(asset, now)
	candles := buildCandles(asset, now, 30)
	prediction := s.buildPrediction(asset, depth, buildFeatures(asset, depth), candles, now)
	return asset, depth, prediction, candles
}

func (s *Service) Depth(market domain.Market, symbol string) (domain.DepthSnapshot, error) {
	snapshot, err := s.Snapshot(market, symbol)
	if err != nil {
		return domain.DepthSnapshot{}, err
	}
	return snapshot.Depth, nil
}

func (s *Service) LatestPrediction(market domain.Market, symbol string) (domain.Prediction, error) {
	snapshot, err := s.Snapshot(market, symbol)
	if err != nil {
		return domain.Prediction{}, err
	}
	return snapshot.Prediction, nil
}

func (s *Service) PredictionHistory(market domain.Market, symbol string) ([]domain.PredictionHistoryItem, error) {
	snapshot, err := s.Snapshot(market, symbol)
	if err != nil {
		return nil, err
	}
	return snapshot.PredictionHistory, nil
}

func (s *Service) buildSnapshot(seed seedAsset, now time.Time) (domain.Asset, domain.DepthSnapshot, domain.Prediction) {
	asset := buildAsset(seed, now)
	depth := buildDepth(asset, now)
	candles := buildCandles(asset, now, 30)
	features := buildFeatures(asset, depth)
	prediction := s.buildPrediction(asset, depth, features, candles, now)
	return asset, depth, prediction
}

func nextPredictionTarget(now time.Time) time.Time {
	return now.UTC().Truncate(predictionCandleInterval).Add(predictionCandleInterval)
}

func (s *Service) cachedPrediction(symbol string, target time.Time) (domain.Prediction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.predictionCache[symbol]
	if !ok || !cached.TargetCandleStart.Equal(target) {
		return domain.Prediction{}, false
	}
	return cached.Prediction, true
}

func (s *Service) rememberPrediction(symbol string, prediction domain.Prediction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.predictionCache[symbol] = cachedPrediction{
		TargetCandleStart: prediction.TargetCandleStart,
		Prediction:        prediction,
	}
}

func (s *Service) buildPrediction(asset domain.Asset, depth domain.DepthSnapshot, features []domain.FeatureAttribution, candles []domain.Candle, now time.Time) domain.Prediction {
	targetCandleStart := nextPredictionTarget(now)
	if cached, ok := s.cachedPrediction(asset.Symbol, targetCandleStart); ok {
		return cached
	}

	featureMap := make(map[string]float64, len(features))
	for _, feature := range features {
		featureMap[feature.Name] = feature.Value
	}
	featureMap["best_bid"] = depth.BestBid
	featureMap["best_ask"] = depth.BestAsk
	featureMap["micro_price"] = depth.MicroPrice
	featureMap["spread_bps_live"] = depth.SpreadBps
	for index := range depth.Bids {
		level := strconv.Itoa(index + 1)
		featureMap["bid_price_"+level] = depth.Bids[index].Price
		featureMap["ask_price_"+level] = depth.Asks[index].Price
		featureMap["bid_orders_"+level] = float64(depth.Bids[index].Orders)
		featureMap["ask_orders_"+level] = float64(depth.Asks[index].Orders)
	}

	var response inference.Response
	if s.inference != nil && s.inference.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		inferenceResponse, err := s.inference.Predict(ctx, inference.Request{
			Symbol:   asset.Symbol,
			Market:   asset.Market,
			Features: featureMap,
		})
		if err == nil {
			response = inferenceResponse
		}
	}

	if response.ModelVersion == "" {
		response = fallbackPrediction(asset, features)
	}

	componentHistory := buildHistoryFromProbabilities(response.UpProbability, response.DownProbability, response.NeutralProbability, now)
	response.ModelComponents = enrichModelComponents(response.ModelComponents, componentHistory, 5)
	consensus := buildConsensusV2(response.ModelComponents)
	regime := classifyRegime(asset, depth, candles)
	confidenceScore := response.ConfidenceScore
	historicalHitRate := response.HistoricalHitRate
	explanation := response.Explanation
	if consensus.Active {
		confidenceScore = round(clamp(confidenceScore+consensus.ConfidenceBoost, 0.4, 0.97), 4)
		historicalHitRate = round(clamp(historicalHitRate+consensus.HitRateBoost, 0.45, 0.9), 4)
		explanation = explanation + " " + consensus.Summary
	}
	if regime == domain.RegimeHighVolatility {
		confidenceScore = round(clamp(confidenceScore-0.05, 0.2, 0.97), 4)
	}
	finalPredictedDirection := finalDirectionFromConsensus(
		predictedDirection(response.UpProbability, response.DownProbability, response.NeutralProbability),
		consensus,
	)
	tradeDecision := s.tradeDecision(asset, depth, consensus, confidenceScore, regime, finalPredictedDirection, response.ModelComponents)

	prediction := domain.Prediction{
		CandleInterval:      "1m",
		PredictionTimestamp: now,
		TargetCandleStart:   targetCandleStart,
		PredictedDirection:  finalPredictedDirection,
		UpProbability:       response.UpProbability,
		DownProbability:     response.DownProbability,
		NeutralProbability:  response.NeutralProbability,
		ConfidenceScore:     confidenceScore,
		SignalLabel:         response.SignalLabel,
		RiskLabel:           response.RiskLabel,
		SignalQuality:       response.SignalQuality,
		HistoricalHitRate:   historicalHitRate,
		ModelVersion:        response.ModelVersion,
		ConsensusActive:     consensus.Active,
		ConsensusDirection:  consensus.Direction,
		ConsensusStrength:   consensus.Strength,
		ConsensusSummary:    consensus.Summary,
		TradeAllowed:        tradeDecision.Allowed,
		TradeAction:         tradeDecision.Action,
		TradeFilterReason:   tradeDecision.Reason,
		RegimeLabel:         regime,
		Explanation:         explanation,
		ModelComponents:     response.ModelComponents,
		TopFeatures:         response.TopFeatures,
		Disclaimer:          disclaimer,
	}
	s.rememberPrediction(asset.Symbol, prediction)
	return prediction
}

func (s *Service) applyTrackedAccuracy(asset domain.Asset, depth domain.DepthSnapshot, prediction domain.Prediction, candles []domain.Candle) (domain.Prediction, []domain.PredictionHistoryItem, domain.AccuracySummary) {
	if s.store != nil {
		if err := s.store.ResolveWithCandles(asset.Symbol, candles); err == nil {
			_ = s.store.UpsertPrediction(asset, depth, prediction)
			history, accuracy, componentRates, summaryErr := s.store.Summary(asset.Symbol, 5, 120)
			if summaryErr == nil {
				history = reconcileHistoryWithCandles(history, candles)
				for index := range prediction.ModelComponents {
					if rates, ok := componentRates[prediction.ModelComponents[index].Name]; ok {
						prediction.ModelComponents[index].HistoricalHitRate = rates.HistoricalHitRate
						prediction.ModelComponents[index].RecentWinRate = rates.RecentWinRate
					}
				}
				if accuracy.LifetimeSampleSize > 0 {
					prediction.HistoricalHitRate = accuracy.LifetimeWinRate
				}
				return prediction, history, accuracy
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tracker := s.trackers[asset.Symbol]
	if tracker == nil {
		tracker = newPredictionTracker()
		s.trackers[asset.Symbol] = tracker
	}

	tracker.observeCandles(candles)
	tracker.recordPrediction(prediction)

	history := tracker.history()
	prediction.ModelComponents = enrichTrackedModelComponents(prediction.ModelComponents, tracker.componentHistory(), 5)
	accuracy := buildAccuracySummary(history, 5)
	if accuracy.SampleSize > 0 {
		prediction.HistoricalHitRate = accuracy.WinRate
	} else {
		prediction.HistoricalHitRate = 0
	}
	return prediction, history, accuracy
}

func reconcileHistoryWithCandles(history []domain.PredictionHistoryItem, candles []domain.Candle) []domain.PredictionHistoryItem {
	if len(history) == 0 || len(candles) == 0 {
		return history
	}

	closed := make(map[int64]domain.Candle, len(candles))
	for _, candle := range candles {
		if candle.IsLive {
			continue
		}
		closed[candle.Timestamp.Truncate(predictionCandleInterval).Unix()] = candle
	}
	if len(closed) == 0 {
		return history
	}

	reconciled := make([]domain.PredictionHistoryItem, 0, len(history))
	for _, item := range history {
		corrected := item
		if candle, ok := closed[item.TargetCandleStart.Truncate(predictionCandleInterval).Unix()]; ok {
			corrected.RealizedDirection = minuteCandleDirection(candle)
			corrected.WasCorrect = corrected.PredictedDirection == corrected.RealizedDirection
		}
		reconciled = append(reconciled, corrected)
	}
	return reconciled
}

type tradeDecision struct {
	Allowed bool
	Action  domain.TradeAction
	Reason  string
}

type cachedPrediction struct {
	TargetCandleStart time.Time
	Prediction        domain.Prediction
}

type symbolTradeThresholds struct {
	MinConfidence     float64
	MaxSpreadBps      float64
	MinDepthBias      float64
	MinMicroPriceBias float64
}

func (s *Service) currentAccuracy(symbol string) domain.AccuracySummary {
	if s.store != nil {
		_, accuracy, _, err := s.store.Summary(symbol, 5, 120)
		if err == nil {
			return accuracy
		}
	}
	return domain.AccuracySummary{}
}

func classifyRegime(asset domain.Asset, depth domain.DepthSnapshot, candles []domain.Candle) domain.RegimeLabel {
	if asset.SignalQuality != domain.SignalQualityFullDepth || depth.SpreadBps > 12 {
		return domain.RegimeLowLiquidity
	}
	if asset.VolatilityScore >= 0.74 {
		return domain.RegimeHighVolatility
	}
	if math.Abs(asset.DepthImbalance-0.5) < 0.08 && math.Abs(asset.MicroPriceBias) < 0.08 {
		return domain.RegimeRange
	}
	if len(candles) >= 4 {
		last := candles[len(candles)-4:]
		upMoves := 0
		downMoves := 0
		for _, candle := range last {
			switch candleDirection(candle) {
			case "up":
				upMoves++
			case "down":
				downMoves++
			}
		}
		if upMoves >= 3 || downMoves >= 3 {
			return domain.RegimeTrend
		}
	}
	return domain.RegimeRange
}

func (s *Service) thresholdsForAsset(asset domain.Asset) symbolTradeThresholds {
	thresholds := symbolTradeThresholds{
		MinConfidence:     s.filter.MinConfidence,
		MaxSpreadBps:      s.filter.MaxSpreadBps,
		MinDepthBias:      s.filter.MinDepthImbalance,
		MinMicroPriceBias: s.filter.MinMicroPriceBias,
	}

	switch strings.ToUpper(asset.Symbol) {
	case "BTCUSDT":
		thresholds.MinConfidence = math.Max(thresholds.MinConfidence, 0.58)
		thresholds.MaxSpreadBps = math.Min(thresholds.MaxSpreadBps, 5.5)
		thresholds.MinDepthBias = math.Max(thresholds.MinDepthBias, 0.035)
		thresholds.MinMicroPriceBias = math.Max(thresholds.MinMicroPriceBias, 0.008)
	case "ETHUSDT":
		thresholds.MinConfidence = math.Max(thresholds.MinConfidence, 0.6)
		thresholds.MaxSpreadBps = math.Min(thresholds.MaxSpreadBps, 7.0)
		thresholds.MinDepthBias = math.Max(thresholds.MinDepthBias, 0.04)
		thresholds.MinMicroPriceBias = math.Max(thresholds.MinMicroPriceBias, 0.01)
	case "SOLUSDT":
		thresholds.MinConfidence = math.Max(thresholds.MinConfidence, 0.63)
		thresholds.MaxSpreadBps = math.Min(thresholds.MaxSpreadBps, 11.0)
		thresholds.MinDepthBias = math.Max(thresholds.MinDepthBias, 0.06)
		thresholds.MinMicroPriceBias = math.Max(thresholds.MinMicroPriceBias, 0.014)
	}

	return thresholds
}

func (s *Service) tradeDecision(asset domain.Asset, depth domain.DepthSnapshot, consensus consensusSummary, confidenceScore float64, regime domain.RegimeLabel, predictedDirection string, components []domain.ModelComponent) tradeDecision {
	if !consensus.Active || consensus.Direction == "" {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Uc model ayni yone bakmiyor."}
	}
	if predictedDirection == "neutral" || consensus.Direction == "neutral" {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Model sonucu yatay; sistem isleme girmiyor."}
	}
	if consensus.Direction != "up" && consensus.Direction != "down" {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Yon kararsiz; sistem isleme girmiyor."}
	}
	if predictedDirection != consensus.Direction {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Nihai tahmin ve model consensus ayni yonde degil."}
	}
	if hasNeutralModelDirection(components) {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Modellerden biri yatay; sistem isleme girmiyor."}
	}
	if !allPrimaryModelsAligned(components, consensus.Direction) {
		return tradeDecision{Action: domain.TradeActionNoTrade, Reason: "Uc model ayni net yone bakmiyor."}
	}

	action := domain.TradeActionNoTrade
	switch consensus.Direction {
	case "up":
		action = domain.TradeActionBuy
	case "down":
		action = domain.TradeActionSell
	}
	return tradeDecision{
		Allowed: true,
		Action:  action,
		Reason:  "Uc model ayni yone bakti ve islem onaylandi.",
	}
}

func (s *Service) inferenceModel() string {
	if s.inference == nil {
		return "depth-fallback-v1"
	}
	return s.inference.Model()
}

var catalog = []seedAsset{
	{Symbol: "AAPL", Name: "Apple", Market: domain.MarketUSEquities, BasePrice: 214, BaseVolume: 4200000, Beta: 0.78, ExchangeCode: "NASDAQ", SessionLabel: "US Regular", Quality: domain.SignalQualityFullDepth},
	{Symbol: "NVDA", Name: "NVIDIA", Market: domain.MarketUSEquities, BasePrice: 938, BaseVolume: 5200000, Beta: 1.2, ExchangeCode: "NASDAQ", SessionLabel: "US Regular", Quality: domain.SignalQualityFullDepth},
	{Symbol: "TSLA", Name: "Tesla", Market: domain.MarketUSEquities, BasePrice: 176, BaseVolume: 6800000, Beta: 1.34, ExchangeCode: "NASDAQ", SessionLabel: "US Regular", Quality: domain.SignalQualityFullDepth},
	{Symbol: "THYAO", Name: "Turkish Airlines", Market: domain.MarketBIST, BasePrice: 312, BaseVolume: 1900000, Beta: 1.12, ExchangeCode: "BIST", SessionLabel: "BIST Continuous", Quality: domain.SignalQualityTopBook},
	{Symbol: "ASELS", Name: "Aselsan", Market: domain.MarketBIST, BasePrice: 68.5, BaseVolume: 2400000, Beta: 0.91, ExchangeCode: "BIST", SessionLabel: "BIST Continuous", Quality: domain.SignalQualityTopBook},
	{Symbol: "KCHOL", Name: "Koc Holding", Market: domain.MarketBIST, BasePrice: 226.4, BaseVolume: 1300000, Beta: 0.72, ExchangeCode: "BIST", SessionLabel: "BIST Continuous", Quality: domain.SignalQualityDegraded},
}

func findSeed(market domain.Market, symbol string) (seedAsset, bool) {
	symbol = strings.ToUpper(symbol)
	for _, item := range catalog {
		if item.Market == market && item.Symbol == symbol {
			return item, true
		}
	}
	return seedAsset{}, false
}

func buildAsset(seed seedAsset, now time.Time) domain.Asset {
	base := float64(now.Unix()) / 10
	h := hash(seed.Symbol)
	drift := wave(base+float64(h), 0.38, seed.BasePrice*0.01*seed.Beta)
	impulse := wave(base+float64(h)/3, 0.93, seed.BasePrice*0.006*seed.Beta)
	price := seed.BasePrice + drift + impulse
	change24h := round(((price-seed.BasePrice)/seed.BasePrice)*100, 2)
	spread := round(0.01+math.Abs(wave(base+float64(h), 0.77, 0.12))*seed.Beta, 3)
	volume := round(seed.BaseVolume*(1+math.Abs(wave(base+float64(h)/2, 0.21, 0.28))), 0)
	depthImbalance := round(clamp(0.5+wave(base+float64(h), 0.44, 0.38), 0.06, 0.94), 2)
	orderFlowImbalance := round(clamp(0.5+wave(base+float64(h)/5, 0.89, 0.42), 0.05, 0.95), 2)
	microPriceBias := round(clamp((depthImbalance-0.5)*2, -1, 1), 2)
	volatility := round(clamp(0.25+math.Abs(change24h)/3.2+seed.Beta*0.08, 0.18, 0.96), 2)
	confidence := round(clamp(0.45+math.Abs(orderFlowImbalance-depthImbalance)+volatility/5, 0.41, 0.93), 2)
	hitRate := round(clamp(0.48+depthImbalance/5+orderFlowImbalance/7-volatility/8, 0.44, 0.81), 2)
	signal := signalLabel(orderFlowImbalance, depthImbalance)
	risk := riskLabel(volatility)

	thesis := "Depth and trade flow are balanced, so the next 1-minute candle is more likely to stay mixed."
	switch signal {
	case domain.SignalStrongBullish:
		thesis = seed.Name + " shows stacked bid depth and aggressive buy-side flow, which biases the next minute upward if liquidity holds."
	case domain.SignalBullish:
		thesis = seed.Name + " has positive near-touch depth pressure and constructive tape, but not enough to remove reversal risk."
	case domain.SignalBearish:
		thesis = seed.Name + " shows weaker bids, ask replenishment, and softer trade flow, which tilts the next minute lower."
	case domain.SignalStrongBearish:
		thesis = seed.Name + " shows sustained ask pressure and fading microprice, which raises downside probability for the next minute."
	}

	return domain.Asset{
		Symbol:              seed.Symbol,
		Name:                seed.Name,
		Market:              seed.Market,
		MarketType:          "stock",
		InstrumentType:      "equity",
		Venue:               strings.ToLower(seed.ExchangeCode),
		SessionLabel:        seed.SessionLabel,
		LastPrice:           round(price, 2),
		ChangePercent24H:    change24h,
		Volume:              volume,
		Spread:              spread,
		SignalLabel:         signal,
		ConfidenceScore:     confidence,
		VolatilityScore:     volatility,
		DepthImbalance:      depthImbalance,
		OrderFlowImbalance:  orderFlowImbalance,
		MicroPriceBias:      microPriceBias,
		HistoricalHitRate:   hitRate,
		NextCandleInterval:  "1m",
		RiskLabel:           risk,
		SignalQuality:       seed.Quality,
		Thesis:              thesis,
		PrimaryExchangeCode: seed.ExchangeCode,
		Range24H: domain.PriceRange{
			Low:  round(price*(1-clamp(volatility*0.03, 0.01, 0.08)), 2),
			High: round(price*(1+clamp(volatility*0.03, 0.01, 0.08)), 2),
		},
	}
}

func buildCryptoAsset(snapshot marketdata.Snapshot) domain.Asset {
	bestBid := snapshot.Depth.BestBid
	bestAsk := snapshot.Depth.BestAsk
	spread := math.Max(bestAsk-bestBid, 0)
	depthImbalance := snapshot.Depth.DepthImbalance
	midPrice := snapshot.LastPrice
	if bestBid > 0 && bestAsk > 0 {
		midPrice = (bestBid + bestAsk) / 2
	}
	microPriceBias := 0.0
	if midPrice > 0 {
		microPriceBias = clamp((snapshot.Depth.MicroPrice-midPrice)/midPrice*180, -1, 1)
	}
	orderFlowImbalance := clamp(0.5+(depthImbalance-0.5)*0.8+clamp(snapshot.ChangePercent24/100, -0.12, 0.12), 0.05, 0.95)
	volatility := round(clamp(math.Abs(snapshot.ChangePercent24)/8+snapshot.Depth.SpreadBps/35, 0.18, 0.96), 2)
	confidence := round(clamp(0.48+math.Abs(orderFlowImbalance-depthImbalance)+math.Abs(microPriceBias)*0.18-volatility*0.06, 0.42, 0.94), 2)
	hitRate := round(clamp(0.49+depthImbalance/4+orderFlowImbalance/8-volatility/10, 0.44, 0.84), 2)
	signal := signalLabel(orderFlowImbalance, depthImbalance)
	risk := riskLabel(volatility)
	thesis := snapshot.Instrument.Name + " order book is balanced and consensus remains mixed."
	switch signal {
	case domain.SignalStrongBullish:
		thesis = snapshot.Instrument.Name + " shows stacked bids, firm microprice support, and constructive depth pressure for the next interval."
	case domain.SignalBullish:
		thesis = snapshot.Instrument.Name + " shows buy-side support in the top levels, but follow-through still depends on spread staying tight."
	case domain.SignalBearish:
		thesis = snapshot.Instrument.Name + " shows softer bids and ask-side replenishment, which tilts the next interval lower."
	case domain.SignalStrongBearish:
		thesis = snapshot.Instrument.Name + " shows sustained ask pressure and fading microprice, increasing downside probability."
	}

	return domain.Asset{
		Symbol:              snapshot.Instrument.Symbol,
		Name:                snapshot.Instrument.Name,
		Market:              domain.MarketCrypto,
		MarketType:          snapshot.Instrument.InstrumentType,
		InstrumentType:      snapshot.Instrument.InstrumentType,
		Venue:               snapshot.Instrument.Venue,
		SessionLabel:        snapshot.Instrument.SessionLabel,
		LastPrice:           round(snapshot.LastPrice, 4),
		ChangePercent24H:    round(snapshot.ChangePercent24, 2),
		Volume:              round(snapshot.Volume, 2),
		Spread:              round(spread, 6),
		SignalLabel:         signal,
		ConfidenceScore:     confidence,
		VolatilityScore:     volatility,
		DepthImbalance:      round(depthImbalance, 4),
		OrderFlowImbalance:  round(orderFlowImbalance, 4),
		MicroPriceBias:      round(microPriceBias, 4),
		HistoricalHitRate:   hitRate,
		NextCandleInterval:  "1m",
		RiskLabel:           risk,
		SignalQuality:       domain.SignalQualityFullDepth,
		Thesis:              thesis,
		PrimaryExchangeCode: snapshot.Instrument.ExchangeCode,
		Range24H: domain.PriceRange{
			Low:  round(snapshot.Low24, 4),
			High: round(snapshot.High24, 4),
		},
	}
}

func buildDepth(asset domain.Asset, now time.Time) domain.DepthSnapshot {
	bids := make([]domain.DepthLevel, 0, 6)
	asks := make([]domain.DepthLevel, 0, 6)
	baseSize := math.Max(asset.Volume/8000, 25)

	for i := 0; i < 6; i++ {
		step := float64(i+1) * math.Max(asset.LastPrice*0.0006, 0.01)
		bidPrice := round(asset.LastPrice-step, 2)
		askPrice := round(asset.LastPrice+step, 2)
		bidSize := round(baseSize*(1+(asset.DepthImbalance-0.5)*1.6)-float64(i)*baseSize*0.05, 0)
		askSize := round(baseSize*(1+(0.5-asset.DepthImbalance)*1.6)-float64(i)*baseSize*0.05, 0)
		if bidSize < 1 {
			bidSize = 1
		}
		if askSize < 1 {
			askSize = 1
		}
		bids = append(bids, domain.DepthLevel{Price: bidPrice, Size: bidSize, Orders: 2 + i})
		asks = append(asks, domain.DepthLevel{Price: askPrice, Size: askSize, Orders: 2 + i})
	}

	bestBid := bids[0].Price
	bestAsk := asks[0].Price
	spreadBps := round(((bestAsk-bestBid)/asset.LastPrice)*10000, 2)
	microPrice := round((bestAsk*bids[0].Size+bestBid*asks[0].Size)/(bids[0].Size+asks[0].Size), 2)

	return domain.DepthSnapshot{
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		SpreadBps:      spreadBps,
		DepthImbalance: asset.DepthImbalance,
		MicroPrice:     microPrice,
		Bids:           bids,
		Asks:           asks,
		LastUpdatedAt:  now,
		SignalQuality:  asset.SignalQuality,
	}
}

func buildFeatures(asset domain.Asset, depth domain.DepthSnapshot) []domain.FeatureAttribution {
	features := []domain.FeatureAttribution{
		{
			Name:         "depth_imbalance",
			Value:        asset.DepthImbalance,
			Contribution: round((asset.DepthImbalance-0.5)*1.7, 2),
			Summary:      "Near-touch bid and ask size balance across the first depth levels.",
		},
		{
			Name:         "order_flow_imbalance",
			Value:        asset.OrderFlowImbalance,
			Contribution: round((asset.OrderFlowImbalance-0.5)*1.6, 2),
			Summary:      "Aggressive buy versus sell flow over the latest minute window.",
		},
		{
			Name:         "microprice_bias",
			Value:        asset.MicroPriceBias,
			Contribution: round(asset.MicroPriceBias*1.4, 2),
			Summary:      "Microprice drift relative to the midpoint and top-of-book pressure.",
		},
		{
			Name:         "spread_bps",
			Value:        depth.SpreadBps,
			Contribution: round(-depth.SpreadBps/24, 2),
			Summary:      "Tighter spread improves short-horizon follow-through reliability.",
		},
	}

	bidTop := 0.0
	askTop := 0.0
	for index := range depth.Bids {
		level := index + 1
		bidSize := depth.Bids[index].Size
		askSize := depth.Asks[index].Size
		if level <= 3 {
			bidTop += bidSize
			askTop += askSize
		}
		levelDenominator := bidSize + askSize
		levelImbalance := 0.0
		if levelDenominator > 0 {
			levelImbalance = (bidSize - askSize) / levelDenominator
		}

		features = append(features,
			domain.FeatureAttribution{
				Name:         "bid_distance_" + strconv.Itoa(level),
				Value:        round(asset.LastPrice-depth.Bids[index].Price, 4),
				Contribution: round(levelImbalance*0.22, 2),
				Summary:      "Bid ladder distance from last price for the first six book levels.",
			},
			domain.FeatureAttribution{
				Name:         "ask_distance_" + strconv.Itoa(level),
				Value:        round(depth.Asks[index].Price-asset.LastPrice, 4),
				Contribution: round(-levelImbalance*0.22, 2),
				Summary:      "Ask ladder distance from last price for the first six book levels.",
			},
			domain.FeatureAttribution{
				Name:         "bid_size_" + strconv.Itoa(level),
				Value:        round(bidSize, 4),
				Contribution: round(levelImbalance*0.46, 2),
				Summary:      "Bid queue size for DeepLOB-style near-touch tensor features.",
			},
			domain.FeatureAttribution{
				Name:         "ask_size_" + strconv.Itoa(level),
				Value:        round(askSize, 4),
				Contribution: round(-levelImbalance*0.46, 2),
				Summary:      "Ask queue size for DeepLOB-style near-touch tensor features.",
			},
			domain.FeatureAttribution{
				Name:         "level_imbalance_" + strconv.Itoa(level),
				Value:        round(levelImbalance, 4),
				Contribution: round(levelImbalance*0.9, 2),
				Summary:      "Per-level order-book imbalance used to score ladder pressure.",
			},
		)
	}

	topDenominator := bidTop + askTop
	queueImbalance := 0.0
	if topDenominator > 0 {
		queueImbalance = (bidTop - askTop) / topDenominator
	}

	shortReturn := round(asset.MicroPriceBias*(asset.Spread/asset.LastPrice)*100, 6)
	realizedVol := round(clamp(asset.VolatilityScore*0.74+math.Abs(asset.ChangePercent24H)/12, 0.04, 1), 4)
	volumePulse := round(clamp((asset.Volume/math.Max(1, asset.LastPrice*12000))-0.8, -1, 1), 4)
	depthPressureTrend := round(clamp((asset.DepthImbalance-0.5)*1.2+(asset.OrderFlowImbalance-0.5)*0.8, -1, 1), 4)
	bookSlope := round((depth.Bids[5].Size-depth.Bids[0].Size-(depth.Asks[5].Size-depth.Asks[0].Size))/math.Max(1, asset.Volume/10000), 4)
	bookPressure := round(clamp(queueImbalance*0.65+asset.MicroPriceBias*0.35, -1, 1), 4)

	features = append(features,
		domain.FeatureAttribution{
			Name:         "queue_imbalance_top3",
			Value:        queueImbalance,
			Contribution: round(queueImbalance*1.2, 2),
			Summary:      "Top-three-level queue imbalance across bid and ask stacks.",
		},
		domain.FeatureAttribution{
			Name:         "book_pressure",
			Value:        bookPressure,
			Contribution: round(bookPressure*1.1, 2),
			Summary:      "Composite book pressure merging queue asymmetry and microprice drift.",
		},
		domain.FeatureAttribution{
			Name:         "depth_pressure_trend",
			Value:        depthPressureTrend,
			Contribution: round(depthPressureTrend*1.05, 2),
			Summary:      "Short-horizon trend of depth pressure and order-flow acceleration.",
		},
		domain.FeatureAttribution{
			Name:         "depth_slope",
			Value:        bookSlope,
			Contribution: round(bookSlope*0.9, 2),
			Summary:      "Shape of the book from the first to sixth level, useful for ladder resilience.",
		},
		domain.FeatureAttribution{
			Name:         "short_return_1m",
			Value:        shortReturn,
			Contribution: round(shortReturn*8, 2),
			Summary:      "Approximate one-minute micro return derived from microprice bias and spread.",
		},
		domain.FeatureAttribution{
			Name:         "realized_vol_8s",
			Value:        realizedVol,
			Contribution: round(-realizedVol*0.72, 2),
			Summary:      "Recent realized volatility proxy over a short rolling window.",
		},
		domain.FeatureAttribution{
			Name:         "volume_pulse",
			Value:        volumePulse,
			Contribution: round(volumePulse*0.66, 2),
			Summary:      "Relative volume expansion used by the FreqAI-style branch.",
		},
	)

	sort.Slice(features, func(i, j int) bool {
		return math.Abs(features[i].Contribution) > math.Abs(features[j].Contribution)
	})
	return features
}

func fallbackPrediction(asset domain.Asset, features []domain.FeatureAttribution) inference.Response {
	up := clamp(0.33+(asset.DepthImbalance-0.5)*0.52+(asset.OrderFlowImbalance-0.5)*0.48+asset.MicroPriceBias*0.12, 0.08, 0.84)
	down := clamp(0.29+(0.5-asset.DepthImbalance)*0.46+(0.5-asset.OrderFlowImbalance)*0.54-asset.MicroPriceBias*0.08, 0.07, 0.83)
	neutral := clamp(1-up-down, 0.06, 0.5)
	total := up + down + neutral
	up /= total
	down /= total
	neutral /= total

	deeplobScore := round((asset.DepthImbalance-0.5)*2, 4)
	deeplobProbability := round(up, 4)
	freqaiScore := round((asset.OrderFlowImbalance-0.5)*2+asset.MicroPriceBias*0.4, 4)
	freqaiProbability := round(clamp(0.5+(asset.OrderFlowImbalance-0.5)*0.8+asset.MicroPriceBias*0.18, 0.08, 0.92), 4)
	tlobScore := round(clamp((asset.DepthImbalance-0.5)*0.9+(asset.OrderFlowImbalance-0.5)*0.7-asset.VolatilityScore*0.18+asset.MicroPriceBias*0.55, -1.2, 1.2), 4)
	tlobProbability := round(clamp(0.5+tlobScore*0.28, 0.08, 0.92), 4)

	return inference.Response{
		UpProbability:      round(up, 4),
		DownProbability:    round(down, 4),
		NeutralProbability: round(neutral, 4),
		PredictedDirection: predictedDirection(up, down, neutral),
		ConfidenceScore:    asset.ConfidenceScore,
		RiskLabel:          asset.RiskLabel,
		SignalLabel:        asset.SignalLabel,
		SignalQuality:      asset.SignalQuality,
		HistoricalHitRate:  asset.HistoricalHitRate,
		ModelVersion:       "deeplob-freqai-tlob-fallback-v3",
		Explanation:        asset.Thesis + " The fallback path blends ladder imbalance, short-horizon FreqAI-style momentum proxies, and a TLOB-style temporal branch when the Python ensemble is unavailable.",
		ModelComponents: []domain.ModelComponent{
			{
				Name:               "DeepLOB depth branch",
				Weight:             0.5,
				Score:              deeplobScore,
				Probability:        deeplobProbability,
				PredictedDirection: componentDirection(deeplobScore, deeplobProbability),
				Summary:            "Depth ladder imbalance, queue pressure, and near-touch shape.",
			},
			{
				Name:               "FreqAI feature branch",
				Weight:             0.3,
				Score:              freqaiScore,
				Probability:        freqaiProbability,
				PredictedDirection: componentDirection(freqaiScore, freqaiProbability),
				Summary:            "Short-horizon flow, microprice, spread, and volatility factors.",
			},
			{
				Name:               "TLOB temporal branch",
				Weight:             0.2,
				Score:              tlobScore,
				Probability:        tlobProbability,
				PredictedDirection: componentDirection(tlobScore, tlobProbability),
				Summary:            "Temporal order-book drift over pressure persistence, volatility, and microprice regime.",
			},
		},
		TopFeatures: features[:minInt(3, len(features))],
	}
}

func predictedDirection(up, down, neutral float64) string {
	if up >= down {
		return "up"
	}
	return "down"
}

func finalDirectionFromConsensus(fallback string, consensus consensusSummary) string {
	if !consensus.Active {
		return "neutral"
	}
	switch consensus.Direction {
	case "up", "down":
		return consensus.Direction
	default:
		return "neutral"
	}
}

type trackedPrediction struct {
	TargetCandleStart  time.Time
	PredictedDirection string
	ConfidenceScore    float64
	ModelDirections    map[string]string
	ConsensusActive    bool
	ConsensusDirection string
	ConsensusStrength  string
	TradeFilterReason  string
	TradeAllowed       bool
	TradeAction        domain.TradeAction
	ResolvedDirection  string
	Resolved           bool
}

type predictionTracker struct {
	pending map[int64]trackedPrediction
	items   []trackedPrediction
}

func newPredictionTracker() *predictionTracker {
	return &predictionTracker{
		pending: make(map[int64]trackedPrediction),
		items:   make([]trackedPrediction, 0, 256),
	}
}

func (t *predictionTracker) observeCandles(candles []domain.Candle) {
	for _, candle := range candles {
		if candle.IsLive {
			continue
		}
		key := candle.Timestamp.Truncate(predictionCandleInterval).Unix()
		pending, ok := t.pending[key]
		if !ok || pending.Resolved {
			continue
		}
		pending.ResolvedDirection = minuteCandleDirection(candle)
		pending.Resolved = true
		t.items = append(t.items, pending)
		delete(t.pending, key)
	}
	if len(t.items) > 400 {
		t.items = t.items[len(t.items)-400:]
	}
}

func (t *predictionTracker) recordPrediction(prediction domain.Prediction) {
	key := prediction.TargetCandleStart.Truncate(predictionCandleInterval).Unix()
	for _, item := range t.items {
		if item.TargetCandleStart.Truncate(predictionCandleInterval).Unix() == key {
			return
		}
	}

	directions := make(map[string]string, len(prediction.ModelComponents))
	for _, component := range prediction.ModelComponents {
		directions[component.Name] = component.PredictedDirection
	}

	t.pending[key] = trackedPrediction{
		TargetCandleStart:  prediction.TargetCandleStart.Truncate(predictionCandleInterval),
		PredictedDirection: prediction.PredictedDirection,
		ConfidenceScore:    prediction.ConfidenceScore,
		ModelDirections:    directions,
		ConsensusActive:    prediction.ConsensusActive,
		ConsensusDirection: prediction.ConsensusDirection,
		ConsensusStrength:  prediction.ConsensusStrength,
		TradeFilterReason:  prediction.TradeFilterReason,
		TradeAllowed:       prediction.TradeAllowed,
		TradeAction:        prediction.TradeAction,
	}
}

func (t *predictionTracker) history() []domain.PredictionHistoryItem {
	items := make([]domain.PredictionHistoryItem, 0, len(t.items))
	for _, item := range t.items {
		items = append(items, domain.PredictionHistoryItem{
			PredictedDirection: item.PredictedDirection,
			RealizedDirection:  item.ResolvedDirection,
			TargetCandleStart:  item.TargetCandleStart,
			ConfidenceScore:    item.ConfidenceScore,
			WasCorrect:         item.PredictedDirection == item.ResolvedDirection,
			TradeAllowed:       item.TradeAllowed,
			TradeAction:        item.TradeAction,
			ModelDirections:    item.ModelDirections,
			ConsensusActive:    item.ConsensusActive,
			ConsensusDirection: item.ConsensusDirection,
			ConsensusStrength:  item.ConsensusStrength,
			TradeFilterReason:  item.TradeFilterReason,
			IsPending:          false,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].TargetCandleStart.Before(items[j].TargetCandleStart)
	})
	return items
}

func (t *predictionTracker) componentHistory() map[string][]domain.PredictionHistoryItem {
	grouped := make(map[string][]domain.PredictionHistoryItem)
	for _, item := range t.items {
		for name, direction := range item.ModelDirections {
			grouped[name] = append(grouped[name], domain.PredictionHistoryItem{
				PredictedDirection: direction,
				RealizedDirection:  item.ResolvedDirection,
				TargetCandleStart:  item.TargetCandleStart,
				ConfidenceScore:    item.ConfidenceScore,
				WasCorrect:         direction == item.ResolvedDirection,
				TradeAllowed:       false,
				TradeAction:        domain.TradeActionNoTrade,
				IsPending:          false,
			})
		}
	}
	return grouped
}

func enrichTrackedModelComponents(components []domain.ModelComponent, componentHistory map[string][]domain.PredictionHistoryItem, recentWindow int) []domain.ModelComponent {
	if len(components) == 0 {
		return components
	}

	enriched := make([]domain.ModelComponent, 0, len(components))
	for _, component := range components {
		history := componentHistory[component.Name]
		if len(history) > 0 {
			component.HistoricalHitRate, component.RecentWinRate = directionalWinRates(history, recentWindow)
		} else {
			component.HistoricalHitRate = 0
			component.RecentWinRate = 0
		}
		enriched = append(enriched, component)
	}
	return enriched
}

func candleDirection(candle domain.Candle) string {
	return resolvedDirection(candle.Open, candle.Close, false)
}

func resolvedDirection(open, close float64, forceDirectional bool) string {
	if forceDirectional {
		if close >= open {
			return "up"
		}
		return "down"
	}

	delta := close - open
	threshold := math.Max(math.Abs(open)*0.00005, 0.000001)
	switch {
	case delta > threshold:
		return "up"
	case delta < -threshold:
		return "down"
	default:
		return "neutral"
	}
}

func minuteCandleDirection(candle domain.Candle) string {
	return minuteDirection(candle.Open, candle.Close)
}

func minuteDirection(open, close float64) string {
	if close >= open {
		return "up"
	}
	return "down"
}

func buildHistoryFromProbabilities(upProbability, downProbability, neutralProbability float64, now time.Time) []domain.PredictionHistoryItem {
	items := make([]domain.PredictionHistoryItem, 0, 8)
	direction := "neutral"
	if upProbability > downProbability && upProbability > neutralProbability {
		direction = "up"
	}
	if downProbability > upProbability && downProbability > neutralProbability {
		direction = "down"
	}

	for i := 8; i >= 1; i-- {
		target := now.Truncate(predictionCandleInterval).Add(-time.Duration(i) * predictionCandleInterval)
		realized := direction
		if i%3 == 0 {
			realized = "neutral"
		}
		if i%4 == 0 {
			if direction == "up" {
				realized = "down"
			} else if direction == "down" {
				realized = "up"
			}
		}
		items = append(items, domain.PredictionHistoryItem{
			PredictedDirection: direction,
			RealizedDirection:  realized,
			TargetCandleStart:  target,
			ConfidenceScore:    clamp(math.Max(upProbability, math.Max(downProbability, neutralProbability))-float64(i)*0.02, 0.36, 0.91),
			WasCorrect:         direction == realized,
			IsPending:          false,
		})
	}
	return items
}

func enrichModelComponents(components []domain.ModelComponent, history []domain.PredictionHistoryItem, recentWindow int) []domain.ModelComponent {
	if len(components) == 0 {
		return components
	}

	enriched := make([]domain.ModelComponent, 0, len(components))
	for _, component := range components {
		if component.PredictedDirection == "" {
			component.PredictedDirection = componentDirection(component.Score, component.Probability)
		}

		componentHistory := historyForDirection(component.PredictedDirection, history)
		component.HistoricalHitRate, component.RecentWinRate = directionalWinRates(componentHistory, recentWindow)
		enriched = append(enriched, component)
	}
	return enriched
}

func historyForDirection(direction string, history []domain.PredictionHistoryItem) []domain.PredictionHistoryItem {
	items := make([]domain.PredictionHistoryItem, 0, len(history))
	for _, item := range history {
		items = append(items, domain.PredictionHistoryItem{
			PredictedDirection: direction,
			RealizedDirection:  item.RealizedDirection,
			TargetCandleStart:  item.TargetCandleStart,
			ConfidenceScore:    item.ConfidenceScore,
			WasCorrect:         direction == item.RealizedDirection,
			IsPending:          false,
		})
	}
	return items
}

func directionalWinRates(history []domain.PredictionHistoryItem, recentWindow int) (float64, float64) {
	if len(history) == 0 {
		return 0, 0
	}

	totalWins := 0
	for _, item := range history {
		if item.WasCorrect {
			totalWins++
		}
	}

	recentCount := minInt(recentWindow, len(history))
	recentWins := 0
	for _, item := range history[len(history)-recentCount:] {
		if item.WasCorrect {
			recentWins++
		}
	}

	return round(float64(totalWins)/float64(len(history)), 2), round(float64(recentWins)/float64(recentCount), 2)
}

func componentDirection(score, probability float64) string {
	if score == 0 {
		if probability >= 0.5 {
			return "up"
		}
		return "down"
	}
	if score > 0 {
		return "up"
	}
	return "down"
}

func componentDirectionalConfidence(component domain.ModelComponent, direction string) float64 {
	switch direction {
	case "up":
		return clamp(component.Probability, 0, 1)
	case "down":
		return clamp(1-component.Probability, 0, 1)
	case "neutral":
		return clamp(1-math.Abs(component.Probability-0.5)*2, 0, 1)
	default:
		return 0
	}
}

func hasNeutralModelDirection(components []domain.ModelComponent) bool {
	for _, component := range components {
		if component.PredictedDirection == "neutral" {
			return true
		}
	}
	return false
}

func allPrimaryModelsAligned(components []domain.ModelComponent, direction string) bool {
	if len(components) < 3 || direction == "" || direction == "neutral" {
		return false
	}
	for _, component := range components[:3] {
		if component.PredictedDirection != direction {
			return false
		}
	}
	return true
}

type consensusSummary struct {
	Active          bool
	Direction       string
	Strength        string
	Summary         string
	ConfidenceBoost float64
	HitRateBoost    float64
}

func buildConsensus(components []domain.ModelComponent) consensusSummary {
	if len(components) < 2 {
		return consensusSummary{}
	}
	first := components[0]
	second := components[1]
	if first.PredictedDirection == "" || first.PredictedDirection != second.PredictedDirection {
		return consensusSummary{
			Summary: "DeepLOB ve FreqAI aynı yöne bakmıyor; bu nedenle sistem ek consensus güçlendirmesi uygulamadan temkinli kalıyor.",
		}
	}

	averageProbability := (first.Probability + second.Probability) / 2
	strength := "aligned"
	confidenceBoost := 0.03
	hitRateBoost := 0.02
	if averageProbability >= 0.62 && first.PredictedDirection != "neutral" {
		strength = "strong"
		confidenceBoost = 0.06
		hitRateBoost = 0.05
	}

	return consensusSummary{
		Active:          true,
		Direction:       first.PredictedDirection,
		Strength:        strength,
		Summary:         "DeepLOB ve FreqAI bir sonraki mum yönünde hizalı; bu nedenle consensus katmanı sınırlı bir güven artışı ekliyor.",
		ConfidenceBoost: confidenceBoost,
		HitRateBoost:    hitRateBoost,
	}
}

func buildConsensusV2(components []domain.ModelComponent) consensusSummary {
	if len(components) < 3 {
		return consensusSummary{}
	}
	counts := map[string]int{}
	probabilitySums := map[string]float64{}
	for _, component := range components[:3] {
		direction := component.PredictedDirection
		if direction == "" {
			continue
		}
		counts[direction]++
		probabilitySums[direction] += componentDirectionalConfidence(component, direction)
	}

	bestDirection := ""
	bestCount := 0
	for _, direction := range []string{"up", "down"} {
		if counts[direction] > bestCount {
			bestDirection = direction
			bestCount = counts[direction]
		}
	}

	if bestDirection == "" {
		return consensusSummary{Summary: "Model yonu olusmadi; consensus pas geciliyor."}
	}
	if bestCount < 3 {
		return consensusSummary{
			Summary: "Tum modeller ayni yone bakmiyor; sistem isleme girmiyor.",
		}
	}
	averageProbability := probabilitySums[bestDirection] / float64(bestCount)
	strength := "aligned"
	confidenceBoost := 0.04
	hitRateBoost := 0.03
	if averageProbability >= 0.62 {
		strength = "strong"
		confidenceBoost = 0.06
		hitRateBoost = 0.05
	}

	return consensusSummary{
		Active:          true,
		Direction:       bestDirection,
		Strength:        strength,
		Summary:         "Uc model de ayni yone bakiyor; consensus katmani yuksek bir guven artisi ekliyor.",
		ConfidenceBoost: confidenceBoost,
		HitRateBoost:    hitRateBoost,
	}
}

func cryptoBasePrice(symbol string) float64 {
	switch strings.ToUpper(symbol) {
	case "BTCUSDT":
		return 68000
	case "ETHUSDT":
		return 3200
	case "SOLUSDT":
		return 145
	default:
		return 100
	}
}

func cryptoBaseVolume(symbol string) float64 {
	switch strings.ToUpper(symbol) {
	case "BTCUSDT":
		return 9500
	case "ETHUSDT":
		return 15000
	case "SOLUSDT":
		return 30000
	default:
		return 12000
	}
}

func cryptoBeta(symbol string) float64 {
	switch strings.ToUpper(symbol) {
	case "BTCUSDT":
		return 1.48
	case "ETHUSDT":
		return 1.35
	case "SOLUSDT":
		return 1.62
	default:
		return 1.2
	}
}

func buildCandles(asset domain.Asset, now time.Time, count int) []domain.Candle {
	points := make([]domain.Candle, 0, count)
	currentMinute := now.UTC().Truncate(predictionCandleInterval)
	h := hash(asset.Symbol)
	for i := 0; i < count; i++ {
		timestamp := currentMinute.Add(-time.Duration(count-1-i) * predictionCandleInterval)
		x := float64(timestamp.Unix()) / 60
		open := round(asset.LastPrice+wave(x+float64(h), 0.72, asset.LastPrice*0.0014), 2)
		close := round(open+wave(x+float64(h)/4, 0.49, asset.LastPrice*0.0011), 2)
		high := round(math.Max(open, close)+math.Abs(wave(x+float64(h), 0.37, asset.LastPrice*0.0008)), 2)
		low := round(math.Min(open, close)-math.Abs(wave(x+float64(h)/6, 0.58, asset.LastPrice*0.0008)), 2)
		points = append(points, domain.Candle{
			Timestamp: timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    round(math.Max(asset.Volume*(0.002+math.Abs(wave(x+float64(h)/8, 0.33, 0.004))), 0), 2),
			IsLive:    i == count-1,
		})
	}
	return points
}

func trimCandles(candles []domain.Candle, count int) []domain.Candle {
	if len(candles) <= count {
		return candles
	}
	return candles[len(candles)-count:]
}

func fallbackChartCandles(base []domain.Candle) map[string][]domain.Candle {
	oneMinute := trimCandles(base, 120)
	return map[string][]domain.Candle{
		"1m_recent": trimCandles(oneMinute, 60),
		"1m":        oneMinute,
		"5m":        aggregateCandles(oneMinute, 5),
		"15m":       aggregateCandles(oneMinute, 15),
		"1h":        aggregateCandles(oneMinute, 60),
		"4h":        aggregateCandles(oneMinute, 240),
		"1d":        aggregateCandles(oneMinute, 1440),
		"1w":        aggregateCandles(oneMinute, 10080),
	}
}

func ensureChartCandles(input map[string][]domain.Candle, fallback []domain.Candle) map[string][]domain.Candle {
	if len(input) == 0 {
		return fallbackChartCandles(fallback)
	}
	if _, ok := input["1m"]; !ok {
		input["1m"] = trimCandles(fallback, 120)
	}
	if _, ok := input["1m_recent"]; !ok {
		input["1m_recent"] = trimCandles(input["1m"], 60)
	}
	if _, ok := input["5m"]; !ok {
		input["5m"] = aggregateCandles(input["1m"], 5)
	}
	if _, ok := input["15m"]; !ok {
		input["15m"] = aggregateCandles(input["1m"], 15)
	}
	if _, ok := input["1h"]; !ok {
		input["1h"] = aggregateCandles(input["1m"], 60)
	}
	if _, ok := input["4h"]; !ok {
		input["4h"] = aggregateCandles(input["1m"], 240)
	}
	if _, ok := input["1d"]; !ok {
		input["1d"] = aggregateCandles(input["1m"], 1440)
	}
	if _, ok := input["1w"]; !ok {
		input["1w"] = aggregateCandles(input["1m"], 10080)
	}
	return input
}

func aggregateCandles(candles []domain.Candle, bucketSize int) []domain.Candle {
	if bucketSize <= 1 || len(candles) == 0 {
		return candles
	}
	out := make([]domain.Candle, 0, (len(candles)/bucketSize)+1)
	for index := 0; index < len(candles); index += bucketSize {
		end := minInt(index+bucketSize, len(candles))
		slice := candles[index:end]
		if len(slice) == 0 {
			continue
		}
		high := slice[0].High
		low := slice[0].Low
		volume := 0.0
		for _, candle := range slice[1:] {
			if candle.High > high {
				high = candle.High
			}
			if candle.Low < low {
				low = candle.Low
			}
		}
		for _, candle := range slice {
			volume += candle.Volume
		}
		out = append(out, domain.Candle{
			Timestamp: slice[0].Timestamp,
			Open:      slice[0].Open,
			High:      high,
			Low:       low,
			Close:     slice[len(slice)-1].Close,
			Volume:    round(volume, 2),
			IsLive:    slice[len(slice)-1].IsLive,
		})
	}
	return out
}

func buildAccuracySummary(history []domain.PredictionHistoryItem, recentWindow int) domain.AccuracySummary {
	history = resolvedHistoryItems(history)
	if len(history) == 0 {
		return domain.AccuracySummary{
			HorizonWinRates:    map[string]float64{},
			HorizonSampleSizes: map[string]int{},
		}
	}

	totalWins := 0
	recentWins := 0
	recentCount := 0
	recent6Wins := 0
	recent6Count := 0
	recent7Wins := 0
	recent7Count := 0
	bullishWins := 0
	bullishTotal := 0
	bearishWins := 0
	bearishTotal := 0
	tradeWins := 0
	tradeTotal := 0
	consensusTradeWins := 0
	consensusTradeTotal := 0
	tradeHistory := make([]domain.PredictionHistoryItem, 0, len(history))

	for _, item := range history {
		if item.WasCorrect {
			totalWins++
		}
		if item.PredictedDirection == "up" {
			bullishTotal++
			if item.WasCorrect {
				bullishWins++
			}
		}
		if item.PredictedDirection == "down" {
			bearishTotal++
			if item.WasCorrect {
				bearishWins++
			}
		}
		if item.TradeAllowed {
			tradeTotal++
			tradeHistory = append(tradeHistory, item)
			if item.WasCorrect {
				tradeWins++
				consensusTradeWins++
			}
			consensusTradeTotal++
		}
	}

	if len(tradeHistory) > 0 {
		recentCount = minInt(recentWindow, len(tradeHistory))
		for _, item := range tradeHistory[len(tradeHistory)-recentCount:] {
			if item.WasCorrect {
				recentWins++
			}
		}
		recent6Count = minInt(6, len(tradeHistory))
		for _, item := range tradeHistory[len(tradeHistory)-recent6Count:] {
			if item.WasCorrect {
				recent6Wins++
			}
		}
		recent7Count = minInt(7, len(tradeHistory))
		for _, item := range tradeHistory[len(tradeHistory)-recent7Count:] {
			if item.WasCorrect {
				recent7Wins++
			}
		}
	}

	currentStreak := 0
	streakDirection := ""
	if len(tradeHistory) > 0 {
		currentStreak = 1
		streakDirection = "win"
		if !tradeHistory[len(tradeHistory)-1].WasCorrect {
			streakDirection = "loss"
		}
		for index := len(tradeHistory) - 2; index >= 0; index-- {
			if tradeHistory[index].WasCorrect == tradeHistory[index+1].WasCorrect {
				currentStreak++
				continue
			}
			break
		}
	}

	bullishAccuracy := 0.0
	if bullishTotal > 0 {
		bullishAccuracy = round(float64(bullishWins)/float64(bullishTotal), 2)
	}
	bearishAccuracy := 0.0
	if bearishTotal > 0 {
		bearishAccuracy = round(float64(bearishWins)/float64(bearishTotal), 2)
	}
	tradeWinRate := 0.0
	if tradeTotal > 0 {
		tradeWinRate = round(float64(tradeWins)/float64(tradeTotal), 2)
	}

	return domain.AccuracySummary{
		WinRate:         round(float64(totalWins)/float64(len(history)), 2),
		LifetimeWinRate: round(float64(totalWins)/float64(len(history)), 2),
		RecentWindowWinRate: func() float64 {
			if recentCount == 0 {
				return 0
			}
			return round(float64(recentWins)/float64(recentCount), 2)
		}(),
		Recent6WinRate: func() float64 {
			if recent6Count == 0 {
				return 0
			}
			return round(float64(recent6Wins)/float64(recent6Count), 2)
		}(),
		Recent7WinRate: func() float64 {
			if recent7Count == 0 {
				return 0
			}
			return round(float64(recent7Wins)/float64(recent7Count), 2)
		}(),
		ConsensusWinRate: 0,
		TradeWinRate:     tradeWinRate,
		TradeSampleSize:  tradeTotal,
		ConsensusTradeWinRate: func() float64 {
			if consensusTradeTotal == 0 {
				return 0
			}
			return round(float64(consensusTradeWins)/float64(consensusTradeTotal), 2)
		}(),
		ConsensusTradeSampleSize: consensusTradeTotal,
		HorizonWinRates: map[string]float64{
			"1m": tradeWinRate,
		},
		HorizonSampleSizes: map[string]int{
			"1m": tradeTotal,
		},
		CurrentStreak:       currentStreak,
		StreakDirection:     streakDirection,
		BullishAccuracy:     bullishAccuracy,
		BearishAccuracy:     bearishAccuracy,
		SampleSize:          len(history),
		LifetimeSampleSize:  len(history),
		ConsensusSampleSize: 0,
	}
}

func resolvedHistoryItems(history []domain.PredictionHistoryItem) []domain.PredictionHistoryItem {
	resolved := make([]domain.PredictionHistoryItem, 0, len(history))
	for _, item := range history {
		if item.IsPending {
			continue
		}
		resolved = append(resolved, item)
	}
	return resolved
}

func signalLabel(flow, depth float64) domain.SignalLabel {
	score := (flow-0.5)*0.58 + (depth-0.5)*0.42
	switch {
	case score > 0.22:
		return domain.SignalStrongBullish
	case score > 0.08:
		return domain.SignalBullish
	case score < -0.22:
		return domain.SignalStrongBearish
	case score < -0.08:
		return domain.SignalBearish
	default:
		return domain.SignalNeutral
	}
}

func riskLabel(volatility float64) domain.RiskLabel {
	switch {
	case volatility > 0.82:
		return domain.RiskHigh
	case volatility > 0.68:
		return domain.RiskElevated
	case volatility > 0.52:
		return domain.RiskMedium
	case volatility > 0.36:
		return domain.RiskGuarded
	default:
		return domain.RiskLow
	}
}

func hash(value string) int {
	total := 0
	for _, char := range value {
		total += int(char)
	}
	return total
}

func wave(base, drift, amplitude float64) float64 {
	return math.Sin(base*drift) * amplitude
}

func clamp(value, minValue, maxValue float64) float64 {
	return math.Min(maxValue, math.Max(minValue, value))
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
