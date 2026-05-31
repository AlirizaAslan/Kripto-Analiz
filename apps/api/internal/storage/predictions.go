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
	PredictionVolume   float64
	ResolvedVolume     float64
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

type symbolIdentity struct {
	Symbol string
	Market domain.Market
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
			prediction_volume REAL NOT NULL DEFAULT 0,
			resolved_volume REAL NOT NULL DEFAULT 0,
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
	recordColumns := map[string]string{
		"trade_allowed":       "INTEGER NOT NULL DEFAULT 0",
		"trade_action":        "TEXT NOT NULL DEFAULT 'no_trade'",
		"trade_filter_reason": "TEXT NOT NULL DEFAULT ''",
		"signal_quality":      "TEXT NOT NULL DEFAULT ''",
		"consensus_strength":  "TEXT NOT NULL DEFAULT ''",
		"regime_label":        "TEXT NOT NULL DEFAULT ''",
		"spread_bps":          "REAL NOT NULL DEFAULT 0",
		"depth_imbalance":     "REAL NOT NULL DEFAULT 0",
		"micro_price_bias":    "REAL NOT NULL DEFAULT 0",
		"volatility_score":    "REAL NOT NULL DEFAULT 0",
		"prediction_hour_utc": "INTEGER NOT NULL DEFAULT 0",
		"prediction_volume":   "REAL NOT NULL DEFAULT 0",
		"resolved_volume":     "REAL NOT NULL DEFAULT 0",
	}
	for name, definition := range recordColumns {
		if err := s.ensurePredictionRecordColumn(name, definition); err != nil {
			return err
		}
	}
	evaluationColumns := map[string]string{
		"trade_allowed":    "INTEGER NOT NULL DEFAULT 0",
		"consensus_active": "INTEGER NOT NULL DEFAULT 0",
	}
	for name, definition := range evaluationColumns {
		if err := s.ensurePredictionEvaluationColumn(name, definition); err != nil {
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

func (s *PredictionStore) UpsertPrediction(asset domain.Asset, depth domain.DepthSnapshot, prediction domain.Prediction, candles []domain.Candle) error {
	modelDirections := make(map[string]string, len(prediction.ModelComponents))
	for _, component := range prediction.ModelComponents {
		modelDirections[component.Name] = component.PredictedDirection
	}
	payload, err := json.Marshal(modelDirections)
	if err != nil {
		return err
	}

	predictionVolume := latestClosedCandleVolume(candles)

	_, err = s.db.Exec(
		`INSERT INTO prediction_records (
			symbol, market, target_candle_start, predicted_direction, confidence_score,
			consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason,
			signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance,
			micro_price_bias, volatility_score, prediction_hour_utc, prediction_volume, model_directions, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			prediction_volume = excluded.prediction_volume,
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
		predictionVolume,
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
			if err := s.resolvePrediction(row.Symbol, row.TargetCandleStart, minuteCandleDirection(candle), candle.Volume); err != nil {
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

func (s *PredictionStore) History(symbol string, historyLimit int) ([]domain.PredictionHistoryItem, error) {
	rows, err := s.resolvedRows(symbol)
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingRows(symbol)
	if err != nil {
		return nil, err
	}
	return buildHistory(rows, pending, historyLimit), nil
}

func (s *PredictionStore) SymbolStatistics(symbol string, recentWindow, historyLimit int) (domain.SymbolStatistics, error) {
	rows, err := s.resolvedRows(symbol)
	if err != nil {
		return domain.SymbolStatistics{}, err
	}
	pending, err := s.pendingRows(symbol)
	if err != nil {
		return domain.SymbolStatistics{}, err
	}
	evals, err := s.resolvedEvaluations(symbol)
	if err != nil {
		return domain.SymbolStatistics{}, err
	}
	market := marketFromRows(rows, pending)
	history := buildHistory(rows, pending, historyLimit)
	hourly := buildHourlyStatistics(rows, pending)
	lastTarget := latestTargetTime(history)
	return domain.SymbolStatistics{
		Symbol:                symbol,
		Market:                market,
		AccuracySummary:       buildAccuracy(rows, evals, recentWindow),
		History:               history,
		HourlyAccuracy:        hourly,
		HourlyInsights:        buildHourlyInsights(hourly),
		TradeFilterBreakdown:  buildTradeFilterBreakdown(rows, pending),
		PendingCount:          len(pending),
		TotalPredictions:      len(rows) + len(pending),
		LastTargetCandleStart: lastTarget,
	}, nil
}

func (s *PredictionStore) AllSymbolStatistics(recentWindow, historyLimit int) ([]domain.SymbolStatistics, error) {
	identities, err := s.symbols()
	if err != nil {
		return nil, err
	}
	items := make([]domain.SymbolStatistics, 0, len(identities))
	for _, identity := range identities {
		item, err := s.SymbolStatistics(identity.Symbol, recentWindow, historyLimit)
		if err != nil {
			continue
		}
		if item.Market == "" {
			item.Market = identity.Market
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Market == items[j].Market {
			return items[i].Symbol < items[j].Symbol
		}
		return items[i].Market < items[j].Market
	})
	return items, nil
}

func (s *PredictionStore) HourlyStatistics(symbol string) ([]domain.HourlyAccuracy, error) {
	rows, err := s.resolvedRows(symbol)
	if err != nil {
		return nil, err
	}
	pending, err := s.pendingRows(symbol)
	if err != nil {
		return nil, err
	}
	return buildHourlyStatistics(rows, pending), nil
}

func (s *PredictionStore) pendingRows(symbol string) ([]predictionRow, error) {
	rows, err := s.db.Query(
		`SELECT symbol, market, target_candle_start, predicted_direction, confidence_score, consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason, signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance, micro_price_bias, volatility_score, prediction_hour_utc, prediction_volume, resolved_volume, model_directions
		FROM prediction_records
		WHERE symbol = ? AND predicted_direction IN ('up', 'down') AND (realized_direction IS NULL OR EXISTS (
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
		`SELECT symbol, market, target_candle_start, predicted_direction, confidence_score, consensus_active, consensus_direction, trade_allowed, trade_action, trade_filter_reason, signal_quality, consensus_strength, regime_label, spread_bps, depth_imbalance, micro_price_bias, volatility_score, prediction_hour_utc, prediction_volume, resolved_volume, realized_direction, was_correct, model_directions
		FROM prediction_records
		WHERE symbol = ? AND predicted_direction IN ('up', 'down') AND realized_direction IS NOT NULL
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
		`SELECT pe.target_candle_start, pe.horizon, pe.was_correct, pe.trade_allowed, pe.consensus_active
		FROM prediction_evaluations pe
		JOIN prediction_records pr ON pe.symbol = pr.symbol AND pe.target_candle_start = pr.target_candle_start
		WHERE pe.symbol = ? AND pr.predicted_direction IN ('up', 'down') AND pe.realized_direction IS NOT NULL`,
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
			&row.PredictionVolume,
			&row.ResolvedVolume,
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
			&row.PredictionVolume,
			&row.ResolvedVolume,
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
	return !target.IsZero()
}

func (s *PredictionStore) resolvePrediction(symbol string, target time.Time, realizedDirection string, resolvedVolume float64) error {
	_, err := s.db.Exec(
		`UPDATE prediction_records
		SET realized_direction = ?, was_correct = CASE WHEN predicted_direction = ? THEN 1 ELSE 0 END, resolved_volume = ?, resolved_at = ?
		WHERE symbol = ? AND target_candle_start = ? AND (realized_direction IS NULL OR (trade_allowed = 1 AND realized_direction = 'neutral'))`,
		realizedDirection,
		realizedDirection,
		resolvedVolume,
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
			ModelDirections:    row.ModelDirections,
			ConsensusActive:    row.ConsensusActive,
			ConsensusDirection: row.ConsensusDirection,
			ConsensusStrength:  row.ConsensusStrength,
			TradeFilterReason:  row.TradeFilterReason,
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
			ModelDirections:    row.ModelDirections,
			ConsensusActive:    row.ConsensusActive,
			ConsensusDirection: row.ConsensusDirection,
			ConsensusStrength:  row.ConsensusStrength,
			TradeFilterReason:  row.TradeFilterReason,
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
			HorizonWinRates:      map[string]float64{},
			HorizonSampleSizes:   map[string]int{},
			RecoveryWrongCandles: []domain.RecoveryWrongCandle{},
			RecoverySteps:        []domain.RecoveryStep{},
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

	stepAttempts := make(map[int]int)
	stepWins := make(map[int]int)
	currentStep := 1

	for _, row := range rows {
		if !row.TradeAllowed {
			continue
		}
		stepAttempts[currentStep]++
		if row.WasCorrect {
			stepWins[currentStep]++
			currentStep = 1
		} else {
			currentStep++
		}
	}

	maxStep := 1
	for step := range stepAttempts {
		if step > maxStep {
			maxStep = step
		}
	}

	recoverySteps := make([]domain.RecoveryStep, 0, maxStep)
	cumulativeWins := 0
	totalSequences := stepAttempts[1]

	for i := 1; i <= maxStep; i++ {
		attempts := stepAttempts[i]
		wins := stepWins[i]
		if attempts == 0 {
			continue
		}

		stepWinRate := round(float64(wins)/float64(attempts), 4)
		cumulativeWins += wins
		cumulativeRate := 0.0
		if totalSequences > 0 {
			cumulativeRate = round(float64(cumulativeWins)/float64(totalSequences), 4)
		}

		recoverySteps = append(recoverySteps, domain.RecoveryStep{
			StepNumber:     i,
			Attempts:       attempts,
			Wins:           wins,
			StepWinRate:    stepWinRate,
			CumulativeWins: cumulativeWins,
			CumulativeRate: cumulativeRate,
		})
	}

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
		RecoverySteps:            recoverySteps,
		RecoveryWrongCandles:     buildRecoveryWrongCandles(rows),
		MaxRecoveryStep:          maxStep,
	}
}

func buildRecoveryWrongCandles(rows []predictionRow) []domain.RecoveryWrongCandle {
	wrongCandles := make([]domain.RecoveryWrongCandle, 0, len(rows))
	currentStep := 1

	for _, row := range rows {
		if !row.TradeAllowed {
			continue
		}
		if currentStep >= 7 && !row.WasCorrect {
			wrongCandles = append(wrongCandles, domain.RecoveryWrongCandle{
				StepNumber:         currentStep,
				TargetCandleStart:  row.TargetCandleStart,
				PredictedDirection: row.PredictedDirection,
				RealizedDirection:  row.RealizedDirection,
				ConfidenceScore:    row.ConfidenceScore,
				TradeAction:        domain.TradeAction(row.TradeAction),
				TradeAllowed:       row.TradeAllowed,
				WasCorrect:         row.WasCorrect,
			})
		}
		if row.WasCorrect {
			currentStep = 1
		} else {
			currentStep++
		}
	}

	return wrongCandles
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

func buildHourlyStatistics(rows []predictionRow, pendingRows []predictionRow) []domain.HourlyAccuracy {
	hourlyMap := make(map[int]*domain.HourlyAccuracy)
	confidenceSums := make(map[int]float64)
	tradeConfidenceSums := make(map[int]float64)
	predictionVolumeSums := make(map[int]float64)
	predictionVolumeCounts := make(map[int]int)
	resolvedVolumeSums := make(map[int]float64)
	resolvedVolumeCounts := make(map[int]int)
	peakVolumes := make(map[int]float64)
	for i := 0; i < 24; i++ {
		hourlyMap[i] = &domain.HourlyAccuracy{Hour: i}
	}

	for _, row := range rows {
		stats := hourlyMap[row.PredictionHourUTC]
		stats.Total++
		confidenceSums[row.PredictionHourUTC] += row.ConfidenceScore
		if row.WasCorrect {
			stats.Wins++
		} else {
			stats.WrongCount++
			stats.WrongVolatilitySum += row.VolatilityScore
		}
		if row.TradeAllowed {
			stats.TradeTotal++
			tradeConfidenceSums[row.PredictionHourUTC] += row.ConfidenceScore
			if row.WasCorrect {
				stats.TradeWins++
			}
		}
		if row.PredictionVolume > 0 {
			predictionVolumeSums[row.PredictionHourUTC] += row.PredictionVolume
			predictionVolumeCounts[row.PredictionHourUTC]++
			if row.PredictionVolume > peakVolumes[row.PredictionHourUTC] {
				peakVolumes[row.PredictionHourUTC] = row.PredictionVolume
			}
		}
		if row.ResolvedVolume > 0 {
			resolvedVolumeSums[row.PredictionHourUTC] += row.ResolvedVolume
			resolvedVolumeCounts[row.PredictionHourUTC]++
			if row.ResolvedVolume > peakVolumes[row.PredictionHourUTC] {
				peakVolumes[row.PredictionHourUTC] = row.ResolvedVolume
			}
		}
	}

	for _, row := range pendingRows {
		stats := hourlyMap[row.PredictionHourUTC]
		stats.Pending++
		if row.PredictionVolume > 0 {
			predictionVolumeSums[row.PredictionHourUTC] += row.PredictionVolume
			predictionVolumeCounts[row.PredictionHourUTC]++
			if row.PredictionVolume > peakVolumes[row.PredictionHourUTC] {
				peakVolumes[row.PredictionHourUTC] = row.PredictionVolume
			}
		}
	}

	var result []domain.HourlyAccuracy
	for i := 0; i < 24; i++ {
		stats := hourlyMap[i]
		if stats.Total > 0 {
			stats.WinRate = round(float64(stats.Wins)/float64(stats.Total), 4)
			stats.AverageConfidence = round(confidenceSums[i]/float64(stats.Total), 4)
		}
		if stats.TradeTotal > 0 {
			stats.TradeWinRate = round(float64(stats.TradeWins)/float64(stats.TradeTotal), 4)
			stats.TradeAverageConfidence = round(tradeConfidenceSums[i]/float64(stats.TradeTotal), 4)
		}
		if stats.WrongCount > 0 {
			stats.AvgWrongVolatility = round(stats.WrongVolatilitySum/float64(stats.WrongCount), 4)
		}
		if predictionVolumeCounts[i] > 0 {
			stats.AveragePredictionVolume = round(predictionVolumeSums[i]/float64(predictionVolumeCounts[i]), 2)
		}
		if resolvedVolumeCounts[i] > 0 {
			stats.AverageResolvedVolume = round(resolvedVolumeSums[i]/float64(resolvedVolumeCounts[i]), 2)
		}
		stats.PeakVolume = round(peakVolumes[i], 2)
		result = append(result, *stats)
	}
	return result
}

func buildHourlyInsights(hourly []domain.HourlyAccuracy) domain.HourlyInsightSet {
	bestHours := hourlyInsightsFrom(hourly, func(item domain.HourlyAccuracy) bool { return item.Total > 0 }, func(item domain.HourlyAccuracy) float64 {
		return item.WinRate
	})
	weakHours := hourlyInsightsFrom(hourly, func(item domain.HourlyAccuracy) bool { return item.Total > 0 }, func(item domain.HourlyAccuracy) float64 {
		return -item.WinRate
	})
	mostActiveHours := hourlyInsightsFrom(hourly, func(item domain.HourlyAccuracy) bool { return item.Total+item.Pending > 0 }, func(item domain.HourlyAccuracy) float64 {
		return float64(item.Total + item.Pending)
	})
	bestTradeHours := hourlyInsightsFrom(hourly, func(item domain.HourlyAccuracy) bool { return item.TradeTotal > 0 }, func(item domain.HourlyAccuracy) float64 {
		return item.TradeWinRate
	})
	highestVolumeHours := hourlyInsightsFrom(hourly, func(item domain.HourlyAccuracy) bool {
		return item.AverageResolvedVolume > 0 || item.AveragePredictionVolume > 0
	}, func(item domain.HourlyAccuracy) float64 {
		if item.AverageResolvedVolume > 0 {
			return item.AverageResolvedVolume
		}
		return item.AveragePredictionVolume
	})

	inactive := make([]int, 0, 24)
	pendingHeavy := make([]int, 0, 24)
	for _, item := range hourly {
		if item.Total == 0 && item.Pending == 0 {
			inactive = append(inactive, item.Hour)
		}
		if item.Pending > item.Total {
			pendingHeavy = append(pendingHeavy, item.Hour)
		}
	}

	return domain.HourlyInsightSet{
		BestHours:          bestHours,
		WeakHours:          weakHours,
		MostActiveHours:    mostActiveHours,
		BestTradeHours:     bestTradeHours,
		HighestVolumeHours: highestVolumeHours,
		InactiveHours:      inactive,
		PendingHeavyHours:  pendingHeavy,
	}
}

func hourlyInsightsFrom(hourly []domain.HourlyAccuracy, include func(domain.HourlyAccuracy) bool, score func(domain.HourlyAccuracy) float64) []domain.HourlyInsight {
	items := make([]domain.HourlyInsight, 0, len(hourly))
	for _, item := range hourly {
		if !include(item) {
			continue
		}
		items = append(items, domain.HourlyInsight{
			Label:         hourLabel(item.Hour),
			Hour:          item.Hour,
			WinRate:       item.WinRate,
			TradeWinRate:  item.TradeWinRate,
			SampleSize:    item.Total,
			TradeSamples:  item.TradeTotal,
			AverageVolume: maxFloat(item.AverageResolvedVolume, item.AveragePredictionVolume),
			PeakVolume:    item.PeakVolume,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := score(hourlyStatFromInsight(items[i], hourly))
		right := score(hourlyStatFromInsight(items[j], hourly))
		if left == right {
			if items[i].SampleSize == items[j].SampleSize {
				return items[i].Hour < items[j].Hour
			}
			return items[i].SampleSize > items[j].SampleSize
		}
		return left > right
	})
	if len(items) > 3 {
		items = items[:3]
	}
	return items
}

func hourlyStatFromInsight(insight domain.HourlyInsight, hourly []domain.HourlyAccuracy) domain.HourlyAccuracy {
	for _, item := range hourly {
		if item.Hour == insight.Hour {
			return item
		}
	}
	return domain.HourlyAccuracy{Hour: insight.Hour}
}

func buildTradeFilterBreakdown(rows []predictionRow, pendingRows []predictionRow) []domain.TradeFilterBreakdown {
	counts := map[string]int{}
	for _, row := range append(append([]predictionRow{}, rows...), pendingRows...) {
		reason := row.TradeFilterReason
		if reason == "" {
			reason = "Islem filtresi tetiklenmedi"
		}
		counts[reason]++
	}
	items := make([]domain.TradeFilterBreakdown, 0, len(counts))
	for reason, count := range counts {
		items = append(items, domain.TradeFilterBreakdown{
			Reason: reason,
			Count:  count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Reason < items[j].Reason
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func latestTargetTime(history []domain.PredictionHistoryItem) *time.Time {
	if len(history) == 0 {
		return nil
	}
	last := history[len(history)-1].TargetCandleStart
	return &last
}

func marketFromRows(rows, pendingRows []predictionRow) domain.Market {
	if len(rows) > 0 {
		return domain.Market(rows[0].Market)
	}
	if len(pendingRows) > 0 {
		return domain.Market(pendingRows[0].Market)
	}
	return ""
}

func hourLabel(hour int) string {
	return time.Date(0, 1, 1, hour, 0, 0, 0, time.UTC).Format("15:04")
}

func latestClosedCandleVolume(candles []domain.Candle) float64 {
	for index := len(candles) - 1; index >= 0; index-- {
		if candles[index].IsLive {
			continue
		}
		return candles[index].Volume
	}
	return 0
}

func (s *PredictionStore) ensurePredictionRecordColumn(name, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(prediction_records);`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE prediction_records ADD COLUMN ` + name + ` ` + definition)
	return err
}

func (s *PredictionStore) ensurePredictionEvaluationColumn(name, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(prediction_evaluations);`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE prediction_evaluations ADD COLUMN ` + name + ` ` + definition)
	return err
}

func (s *PredictionStore) symbols() ([]symbolIdentity, error) {
	rows, err := s.db.Query(`SELECT symbol, market FROM prediction_records GROUP BY symbol, market ORDER BY market, symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]symbolIdentity, 0, 16)
	for rows.Next() {
		var item symbolIdentity
		var market string
		if err := rows.Scan(&item.Symbol, &market); err != nil {
			return nil, err
		}
		item.Market = domain.Market(market)
		items = append(items, item)
	}
	return items, rows.Err()
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
