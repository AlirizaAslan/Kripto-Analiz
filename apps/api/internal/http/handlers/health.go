package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"pulsealpha/api/internal/config"
)

func Health(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inference := "fallback_only"
		if cfg.InferenceURL != "" {
			inference = "configured"
		}
		marketData := "synthetic_provider_scaffold"
		if len(cfg.BinanceSpotSymbols) > 0 || len(cfg.BinanceFuturesSymbols) > 0 {
			marketData = "binance_public_depth_configured"
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"timestamp": time.Now().UTC(),
			"services": map[string]string{
				"marketData": marketData,
				"inference":  inference,
			},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
