package handlers

import (
	"net/http"
	"strings"

	"pulsealpha/api/internal/domain"
	"pulsealpha/api/internal/market"
)

func MarketOverview(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, service.Overview())
	}
}

func SymbolSnapshot(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		marketName, ok := parseMarket(r.PathValue("market"))
		if !ok {
			http.Error(w, "unsupported market", http.StatusBadRequest)
			return
		}

		snapshot, err := service.Snapshot(marketName, r.PathValue("symbol"))
		if err != nil {
			http.Error(w, "symbol not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, snapshot)
	}
}

func Depth(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		marketName, ok := parseMarket(r.PathValue("market"))
		if !ok {
			http.Error(w, "unsupported market", http.StatusBadRequest)
			return
		}

		depth, err := service.Depth(marketName, r.PathValue("symbol"))
		if err != nil {
			http.Error(w, "symbol not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, depth)
	}
}

func LatestPrediction(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		marketName, ok := parseMarket(r.PathValue("market"))
		if !ok {
			http.Error(w, "unsupported market", http.StatusBadRequest)
			return
		}

		prediction, err := service.LatestPrediction(marketName, r.PathValue("symbol"))
		if err != nil {
			http.Error(w, "symbol not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, prediction)
	}
}

func PredictionHistory(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		marketName, ok := parseMarket(r.PathValue("market"))
		if !ok {
			http.Error(w, "unsupported market", http.StatusBadRequest)
			return
		}

		history, err := service.PredictionHistory(marketName, r.PathValue("symbol"))
		if err != nil {
			http.Error(w, "symbol not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"items": history,
		})
	}
}

func SymbolStatistics(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		marketName, ok := parseMarket(r.PathValue("market"))
		if !ok {
			http.Error(w, "unsupported market", http.StatusBadRequest)
			return
		}

		stats, err := service.SymbolStatistics(marketName, r.PathValue("symbol"))
		if err != nil {
			http.Error(w, "symbol not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, stats)
	}
}

func AllStatistics(service *market.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := service.AllSymbolStatistics()
		if err != nil {
			http.Error(w, "statistics not available", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

func parseMarket(value string) (domain.Market, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(domain.MarketUSEquities):
		return domain.MarketUSEquities, true
	case string(domain.MarketBIST):
		return domain.MarketBIST, true
	case string(domain.MarketCrypto):
		return domain.MarketCrypto, true
	default:
		return "", false
	}
}
