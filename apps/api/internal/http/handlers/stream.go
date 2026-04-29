package handlers

import (
	"net/http"

	"pulsealpha/api/internal/realtime"
)

func Stream(hub *realtime.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		// A real implementation upgrades the connection and authorizes channel access
		// from the server-side identity instead of trusting any client-provided user id.
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"message": "websocket upgrade is reserved for authenticated quote, depth, candle_1m, and prediction_1m streams",
		})
		_ = hub
	}
}
