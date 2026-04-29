package storage

import (
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"pulsealpha/api/internal/domain"
)

var evaluationHorizons = map[string]time.Duration{
	"1m": time.Minute,
}

const predictionCandleInterval = time.Minute

type PredictionStore struct {
	db *sql.DB
}

type predictionRow struct {
	Symbol             string
	Market             string
	TargetCandleStart  time.Time
	PredictedDirection string
	RealizedDirection  string
	ConfidenceScore    float64
	WasCorrect         bool
	ConsensusActive    bool
	ConsensusDirection string
	TradeAllowed       bool
	TradeAction        string
	TradeFilterReason  string
	SignalQuality      string
	ConsensusStrength  string
	RegimeLabel        string
	SpreadBps          float64
	DepthImbalance     float64
	MicroPriceBias     float64
	VolatilityScore    float64
	PredictionHourUTC  int
	ModelDirections    map[string]string
}

type evaluationRow struct {
	TargetCandleStart time.Time
	Horizon           string
	WasCorrect        bool
	TradeAllowed      bool
	ConsensusActive   bool
}

type ComponentAccuracy struct {
	HistoricalHitRate float64
	RecentWinRate     float64
}

func OpenPredictionStore(path string) (*PredictionStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &PredictionStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PredictionStore) init() error {
	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS prediction_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			market TEXT NOT NULL,
			target_candle_start TEXT NOT NULL,
			predicted_direction TEXT NOT NULL,
			confidence_score REAL NOT NULL,
			consensus_active INTEGER NOT NULL,
			consensus_direction TEXT NOT NULL,
			trade_allowed INTEGER NOT NULL DEFAULT 0,
			trade_action TEXT NOT NULL DEFAULT 'no_trade',
			trade_filter_reason TEXT NOT NULL DEFAULT '',
			signal_quality TEXT NOT NULL DEFAULT '',
			consensus_strength TEXT NOT NULL DEFAULT '',
			regime_label TEXT NOT NULL DEFAULT '',
			spread_bps REAL NOT NULL DEFAULT 0,
			depth_imbalance REAL NOT NULL DEFAULT 0,
			micro_price_bias REAL NOT NULL DEFAULT 0,
			volatility_score REAL NOT NULL DEFAULT 0,
			prediction_hour_utc INTEGER NOT NULL DEFAULT 0,
			realized_direction TEXT,
			was_correct INTEGER,
			resolved_at TEXT,
			model_directions TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(symbol, target_candle_start)
		);`,
		`CREATE TABLE IF NOT EXISTS prediction_evaluations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			target_candle_start TEXT NOT NULL,
			horizon TEXT NOT NULL,
			trade_allowed INTEGER NOT NULL DEFAULT 0,
			consensus_active INTEGER NOT NULL DEFAULT 0,
			realized_direction TEXT,
			was_correct INTEGER,
			resolved_at TEXT,
			UNIQUE(symbol, target_candle_start, horizon)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_prediction_records_symbol_target ON prediction_records(symbol, target_candle_start);`,
		`CREATE INDEX IF NOT EXISTS idx_prediction_records_symbol_resolved ON prediction_records(symbol, resolved_at);`,
		`CREATE INDEX IF NOT EXISTS idx_prediction_evaluations_symbol_target ON prediction_evaluations(symbol, target_candle_start);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *PredictionStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PredictionStore) UpsertPrediction(asset domain.Asset, depth domain.DepthSnapshot, prediction domain.Prediction) error {
	modelDirections := make(map[string]string, len(prediction.ModelComponents))
	for _, component := range prediction.ModelComponents {
		modelDirections[component.Name] = component.PredictedDirection
	}
	payload, err := json.Marshal(modelDirections)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO prediction_records (
			symbol, market, target_candle_start, predicted_direction, confidence_score,
			consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason,
			signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance,
			micro_price_bias, volatility_score, prediction_hour_utc, model_directions, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, target_candle_start) DO UPDATE SET
			market = excluded.market,
			predicted_direction = excluded.predicted_direction,
			confidence_score = excluded.confidence_score,
			consensus_active = excluded.consensus_active,
			consensus_direction = excluded.consensus_direction,
			trade_allowed = excluded.trade_allowed,
			trade_action = excluded.trade_action,
			trade_filter_reason = excluded.trade_filter_reason,
			signal_quality = excluded.signal_quality,
			consensus_strength = excluded.consensus_strength,
			regime_label = excluded.regime_label,
			spread_bps = excluded.spread_bps,
			depth_imbalance = excluded.depth_imbalance,
			micro_price_bias = excluded.micro_price_bias,
			volatility_score = excluded.volatility_score,
			prediction_hour_utc = excluded.prediction_hour_utc,
			model_directions = excluded.model_directions`,
		asset.Symbol,
		string(asset.Market),
		prediction.TargetCandleStart.UTC().Format(time.RFC3339Nano),
		prediction.PredictedDirection,
		prediction.ConfidenceScore,
		boolToInt(prediction.ConsensusActive),
		prediction.ConsensusDirection,
		boolToInt(prediction.TradeAllowed),
		string(prediction.TradeAction),
		prediction.TradeFilterReason,
		string(prediction.SignalQuality),
		prediction.ConsensusStrength,
		string(prediction.RegimeLabel),
		depth.SpreadBps,
		depth.DepthImbalance,
		asset.MicroPriceBias,
		asset.VolatilityScore,
		prediction.PredictionTimestamp.UTC().Hour(),
		string(payload),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return err
	}

	for horizon := range evaluationHorizons {
		if _, err := s.db.Exec(
			`INSERT INTO prediction_evaluations (
				symbol, target_candle_start, horizon, trade_allowed, consensus_active
			) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(symbol, target_candle_start, horizon) DO UPDATE SET
				trade_allowed = excluded.trade_allowed,
				consensus_active = excluded.consensus_active`,
			asset.Symbol,
			prediction.TargetCandleStart.UTC().Format(time.RFC3339Nano),
			horizon,
			boolToInt(prediction.TradeAllowed),
			boolToInt(prediction.ConsensusActive),
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *PredictionStore) ResolveWithCandles(symbol string, candles []domain.Candle) error {
	closed := make(map[int64]domain.Candle, len(candles))
	for _, candle := range candles {
		if candle.IsLive {
			continue
		}
		closed[candle.Timestamp.Truncate(predictionCandleInterval).Unix()] = candle
	}
	if len(closed) == 0 {
		return nil
	}

	rows, err := s.pendingRows(symbol)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if candle, ok := closed[row.TargetCandleStart.Unix()]; ok {
			if err := s.resolvePrediction(row.Symbol, row.TargetCandleStart, minuteCandleDirection(candle)); err != nil {
				return err
			}
		}
		for horizon, duration := range evaluationHorizons {
			endTs := row.TargetCandleStart.Add(duration).Add(-predictionCandleInterval).Unix()
			startCandle, startOk := closed[row.TargetCandleStart.Unix()]
			endCandle, endOk := closed[endTs]
			if !startOk || !endOk {
				continue
			}
			if err := s.resolveEvaluation(row.Symbol, row.TargetCandleStart, horizon, minuteDirection(startCandle.Open, endCandle.Close)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PredictionStore) Summary(symbol string, recentWindow, historyLimit int) ([]domain.PredictionHistoryItem, domain.AccuracySummary, map[string]ComponentAccuracy, error) {
	rows, err := s.resolvedRows(symbol)
	if err != nil {
		return nil, domain.AccuracySummary{}, nil, err
	}
	pending, err := s.pendingRows(symbol)
	if err != nil {
		return nil, domain.AccuracySummary{}, nil, err
	}
	evals, err := s.resolvedEvaluations(symbol)
	if err != nil {
		return nil, domain.AccuracySummary{}, nil, err
	}

	history := buildHistory(rows, pending, historyLimit)
	accuracy := buildAccuracy(rows, evals, recentWindow)
	componentRates := buildComponentRates(rows, recentWindow)
	return history, accuracy, componentRates, nil
}

func (s *PredictionStore) pendingRows(symbol string) ([]predictionRow, error) {
	rows, err := s.db.Query(
		`SELECT symbol, market, target_candle_start, predicted_direction, confidence_score, consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason, signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance, micro_price_bias, volatility_score, prediction_hour_utc, model_directions
		FROM prediction_records
		WHERE symbol = ? AND (realized_direction IS NULL OR EXISTS (
			SELECT 1 FROM prediction_evaluations pe WHERE pe.symbol = prediction_records.symbol AND pe.target_candle_start = prediction_records.target_candle_start AND pe.realized_direction IS NULL
		) OR (trade_allowed = 1 AND realized_direction = 'neutral') OR EXISTS (
			SELECT 1 FROM prediction_evaluations pe WHERE pe.symbol = prediction_records.symbol AND pe.target_candle_start = prediction_records.target_candle_start AND pe.trade_allowed = 1 AND pe.realized_direction = 'neutral'
		))
		ORDER BY target_candle_start ASC`,
		symbol,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]predictionRow, 0, 64)
	for rows.Next() {
		row, err := scanPredictionRow(rows, false)
		if err != nil {
			return nil, err
		}
		if !isPredictionTargetAligned(row.TargetCandleStart) {
			continue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PredictionStore) resolvedRows(symbol string) ([]predictionRow, error) {
	rows, err := s.db.Query(
		`SELECT symbol, market, target_candle_start, predicted_direction, confidence_score, consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason, signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance, micro_price_bias, volatility_score, prediction_hour_utc, realized_direction, was_correct, model_directions
		FROM prediction_records
		WHERE symbol = ? AND realized_direction IS NOT NULL
		ORDER BY target_candle_start ASC`,
		symbol,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]predictionRow, 0, 128)
	for rows.Next() {
		row, err := scanPredictionRow(rows, true)
		if err != nil {
			return nil, err
		}
		if !isPredictionTargetAligned(row.TargetCandleStart) {
			continue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PredictionStore) resolvedEvaluations(symbol string) ([]evaluationRow, error) {
	rows, err := s.db.Query(
		`SELECT target_candle_start, horizon, was_correct, trade_allowed, consensus_active
		FROM prediction_evaluations
		WHERE symbol = ? AND realized_direction IS NOT NULL`,
		symbol,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]evaluationRow, 0, 256)
	for rows.Next() {
		var row evaluationRow
		var target string
		var wasCorrect int
		var tradeAllowed int
		var consensusActive int
		if err := rows.Scan(&target, &row.Horizon, &wasCorrect, &tradeAllowed, &consensusActive); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, target)
		if err != nil {
			return nil, err
		}
		if !isPredictionTargetAligned(parsed) {
			continue
		}
		row.TargetCandleStart = parsed
		row.WasCorrect = wasCorrect == 1
		row.TradeAllowed = tradeAllowed == 1
		row.ConsensusActive = consensusActive == 1
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanPredictionRow(scanner interface {
	Scan(dest ...any) error
}, resolved bool) (predictionRow, error) {
	row := predictionRow{}
	var (
		target             string
		modelDirectionsRaw string
		consensusActive    int
		tradeAllowed       int
		wasCorrect         sql.NullInt64
		realizedDirection  sql.NullString
	)

	if resolved {
		err := scanner.Scan(
			&row.Symbol,
			&row.Market,
			&target,
			&row.PredictedDirection,
			&row.ConfidenceScore,
			&consensusActive,
			&row.ConsensusDirection,
			&tradeAllowed,
			&row.TradeAction,
			&row.TradeFilterReason,
			&row.SignalQuality,
			&row.ConsensusStrength,
			&row.RegimeLabel,
			&row.SpreadBps,
			&row.DepthImbalance,
			&row.MicroPriceBias,
			&row.VolatilityScore,
			&row.PredictionHourUTC,
			&realizedDirection,
			&wasCorrect,
			&modelDirectionsRaw,
		)
		if err != nil {
			return row, err
		}
		if realizedDirection.Valid {
			row.RealizedDirection = realizedDirection.String
		}
		if wasCorrect.Valid {
			row.WasCorrect = wasCorrect.Int64 == 1
		}
	} else {
		err := scanner.Scan(
			&row.Symbol,
			&row.Market,
			&target,
			&row.PredictedDirection,
			&row.ConfidenceScore,
			&consensusActive,
			&row.ConsensusDirection,
			&tradeAllowed,
			&row.TradeAction,
			&row.TradeFilterReason,
			&row.SignalQuality,
			&row.ConsensusStrength,
			&row.RegimeLabel,
			&row.SpreadBps,
			&row.DepthImbalance,
			&row.MicroPriceBias,
			&row.VolatilityScore,
			&row.PredictionHourUTC,
			&modelDirectionsRaw,
		)
		if err != nil {
			return row, err
		}
	}

	parsed, err := time.Parse(time.RFC3339Nano, target)
	if err != nil {
		return row, err
	}
	row.TargetCandleStart = parsed
	row.ConsensusActive = consensusActive == 1
	row.TradeAllowed = tradeAllowed == 1
	if err := json.Unmarshal([]byte(modelDirectionsRaw), &row.ModelDirections); err != nil {
		return row, err
	}
	return row, nil
}

func isPredictionTargetAligned(target time.Time) bool {
	return target.Equal(target.Truncate(predictionCandleInterval))
}

func (s *PredictionStore) resolvePrediction(symbol string, target time.Time, realizedDirection string) error {
	_, err := s.db.Exec(
		`UPDATE prediction_records
		SET realized_direction = ?, was_correct = CASE WHEN predicted_direction = ? THEN 1 ELSE 0 END, resolved_at = ?
		WHERE symbol = ? AND target_candle_start = ? AND (realized_direction IS NULL OR (trade_allowed = 1 AND realized_direction = 'neutral'))`,
		realizedDirection,
		realizedDirection,
		time.Now().UTC().Format(time.RFC3339Nano),
		symbol,
		target.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *PredictionStore) resolveEvaluation(symbol string, target time.Time, horizon string, realizedDirection string) error {
	_, err := s.db.Exec(
		`UPDATE prediction_evaluations
		SET realized_direction = ?, was_correct = (
			SELECT CASE WHEN predicted_direction = ? THEN 1 ELSE 0 END
			FROM prediction_records
			WHERE symbol = ? AND target_candle_start = ?
		), resolved_at = ?
		WHERE symbol = ? AND target_candle_start = ? AND horizon = ? AND (realized_direction IS NULL OR (trade_allowed = 1 AND realized_direction = 'neutral'))`,
		realizedDirection,
		realizedDirection, symbol, target.UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
		symbol,
		target.UTC().Format(time.RFC3339Nano),
		horizon,
	)
	return err
}

func buildHistory(rows []predictionRow, pendingRows []predictionRow, historyLimit int) []domain.PredictionHistoryItem {
	items := make([]domain.PredictionHistoryItem, 0, len(rows)+len(pendingRows))
	resolvedTargets := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		resolvedTargets[row.TargetCandleStart.Truncate(predictionCandleInterval).Unix()] = struct{}{}
		items = append(items, domain.PredictionHistoryItem{
			PredictedDirection: row.PredictedDirection,
			RealizedDirection:  row.RealizedDirection,
			TargetCandleStart:  row.TargetCandleStart,
			ConfidenceScore:    row.ConfidenceScore,
			WasCorrect:         row.WasCorrect,
			TradeAllowed:       row.TradeAllowed,
			TradeAction:        domain.TradeAction(row.TradeAction),
		})
	}
	for _, row := range pendingRows {
		key := row.TargetCandleStart.Truncate(predictionCandleInterval).Unix()
		if _, ok := resolvedTargets[key]; ok {
			continue
		}
		items = append(items, domain.PredictionHistoryItem{
			PredictedDirection: row.PredictedDirection,
			RealizedDirection:  "",
			TargetCandleStart:  row.TargetCandleStart,
			ConfidenceScore:    row.ConfidenceScore,
			WasCorrect:         false,
			TradeAllowed:       row.TradeAllowed,
			TradeAction:        domain.TradeAction(row.TradeAction),
			IsPending:          true,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].TargetCandleStart.Before(items[j].TargetCandleStart)
	})
	if historyLimit > 0 && len(items) > historyLimit {
		items = items[len(items)-historyLimit:]
	}
	return items
}

func buildAccuracy(rows []predictionRow, evals []evaluationRow, recentWindow int) domain.AccuracySummary {
	if len(rows) == 0 {
		return domain.AccuracySummary{
			HorizonWinRates:    map[string]float64{},
			HorizonSampleSizes: map[string]int{},
		}
	}

	totalWins := 0
	bullishWins := 0
	bullishTotal := 0
	bearishWins := 0
	bearishTotal := 0
	consensusWins := 0
	consensusTotal := 0
	tradeWins := 0
	tradeTotal := 0
	consensusTradeWins := 0
	consensusTradeTotal := 0

	for _, row := range rows {
		if row.WasCorrect {
			totalWins++
		}
		if row.PredictedDirection == "up" {
			bullishTotal++
			if row.WasCorrect {
				bullishWins++
			}
		}
		if row.PredictedDirection == "down" {
			bearishTotal++
			if row.WasCorrect {
				bearishWins++
			}
		}
		if row.ConsensusActive {
			consensusTotal++
			if row.WasCorrect {
				consensusWins++
			}
		}
		if row.TradeAllowed {
			tradeTotal++
			if row.WasCorrect {
				tradeWins++
			}
			if row.ConsensusActive {
				consensusTradeTotal++
				if row.WasCorrect {
					consensusTradeWins++
				}
			}
		}
	}

	recentCount := recentWindow
	if recentCount > len(rows) {
		recentCount = len(rows)
	}
	recentWins := 0
	for _, row := range rows[len(rows)-recentCount:] {
		if row.WasCorrect {
			recentWins++
		}
	}

	recent6Count := 6
	if recent6Count > len(rows) {
		recent6Count = len(rows)
	}
	recent6Wins := 0
	for _, row := range rows[len(rows)-recent6Count:] {
		if row.WasCorrect {
			recent6Wins++
		}
	}

	recent7Count := 7
	if recent7Count > len(rows) {
		recent7Count = len(rows)
	}
	recent7Wins := 0
	for _, row := range rows[len(rows)-recent7Count:] {
		if row.WasCorrect {
			recent7Wins++
		}
	}

	currentStreak := 1
	streakDirection := "win"
	if !rows[len(rows)-1].WasCorrect {
		streakDirection = "loss"
	}
	for index := len(rows) - 2; index >= 0; index-- {
		if rows[index].WasCorrect == rows[index+1].WasCorrect {
			currentStreak++
			continue
		}
		break
	}

	horizonWins := map[string]int{}
	horizonTotals := map[string]int{}
	for _, item := range evals {
		horizonTotals[item.Horizon]++
		if item.WasCorrect {
			horizonWins[item.Horizon]++
		}
	}
	horizonRates := make(map[string]float64, len(horizonTotals))
	for horizon, total := range horizonTotals {
		if total > 0 {
			horizonRates[horizon] = round(float64(horizonWins[horizon])/float64(total), 4)
		}
	}

	bullishAccuracy := 0.0
	if bullishTotal > 0 {
		bullishAccuracy = round(float64(bullishWins)/float64(bullishTotal), 4)
	}
	bearishAccuracy := 0.0
	if bearishTotal > 0 {
		bearishAccuracy = round(float64(bearishWins)/float64(bearishTotal), 4)
	}
	consensusWinRate := 0.0
	if consensusTotal > 0 {
		consensusWinRate = round(float64(consensusWins)/float64(consensusTotal), 4)
	}
	tradeWinRate := 0.0
	if tradeTotal > 0 {
		tradeWinRate = round(float64(tradeWins)/float64(tradeTotal), 4)
	}
	consensusTradeWinRate := 0.0
	if consensusTradeTotal > 0 {
		consensusTradeWinRate = round(float64(consensusTradeWins)/float64(consensusTradeTotal), 4)
	}
	winRate := round(float64(totalWins)/float64(len(rows)), 4)

	return domain.AccuracySummary{
		WinRate:                  winRate,
		LifetimeWinRate:          winRate,
		RecentWindowWinRate:      round(float64(recentWins)/float64(recentCount), 4),
		Recent6WinRate:           round(float64(recent6Wins)/float64(recent6Count), 4),
		Recent7WinRate:           round(float64(recent7Wins)/float64(recent7Count), 4),
		ConsensusWinRate:         consensusWinRate,
		TradeWinRate:             tradeWinRate,
		TradeSampleSize:          tradeTotal,
		ConsensusTradeWinRate:    consensusTradeWinRate,
		ConsensusTradeSampleSize: consensusTradeTotal,
		HorizonWinRates:          horizonRates,
		HorizonSampleSizes:       horizonTotals,
		CurrentStreak:            currentStreak,
		StreakDirection:          streakDirection,
		BullishAccuracy:          bullishAccuracy,
		BearishAccuracy:          bearishAccuracy,
		SampleSize:               len(rows),
		LifetimeSampleSize:       len(rows),
		ConsensusSampleSize:      consensusTotal,
	}
}

func buildComponentRates(rows []predictionRow, recentWindow int) map[string]ComponentAccuracy {
	type aggregate struct {
		total  int
		wins   int
		recent []bool
	}
	aggregates := make(map[string]*aggregate)

	for _, row := range rows {
		for name, direction := range row.ModelDirections {
			current := aggregates[name]
			if current == nil {
				current = &aggregate{recent: make([]bool, 0, recentWindow)}
				aggregates[name] = current
			}
			current.total++
			correct := direction == row.RealizedDirection
			if correct {
				current.wins++
			}
			current.recent = append(current.recent, correct)
			if len(current.recent) > recentWindow {
				current.recent = current.recent[len(current.recent)-recentWindow:]
			}
		}
	}

	out := make(map[string]ComponentAccuracy, len(aggregates))
	for name, item := range aggregates {
		recentWins := 0
		for _, wasCorrect := range item.recent {
			if wasCorrect {
				recentWins++
			}
		}
		recentRate := 0.0
		if len(item.recent) > 0 {
			recentRate = round(float64(recentWins)/float64(len(item.recent)), 4)
		}
		out[name] = ComponentAccuracy{
			HistoricalHitRate: round(float64(item.wins)/float64(item.total), 4),
			RecentWinRate:     recentRate,
		}
	}
	return out
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func candleDirection(candle domain.Candle) string {
	return horizonDirection(candle.Open, candle.Close)
}

func resolvedDirection(open, close float64, forceDirectional bool) string {
	if forceDirectional {
		if close >= open {
			return "up"
		}
		return "down"
	}
	return horizonDirection(open, close)
}

func horizonDirection(open, close float64) string {
	delta := close - open
	threshold := maxFloat(absFloat(open)*0.00005, 0.000001)
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

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
