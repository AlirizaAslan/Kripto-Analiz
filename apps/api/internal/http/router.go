package http

import (
	"log"
	"net/http"

	"pulsealpha/api/internal/config"
	"pulsealpha/api/internal/http/handlers"
	"pulsealpha/api/internal/http/middleware"
	"pulsealpha/api/internal/inference"
	"pulsealpha/api/internal/market"
	"pulsealpha/api/internal/marketdata"
	"pulsealpha/api/internal/realtime"
	"pulsealpha/api/internal/storage"
)

func NewRouter(cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	hub := realtime.NewHub()
	provider := marketdata.NewBinanceProvider(cfg.BinanceSpotURL, cfg.BinanceFuturesURL, cfg.BinanceSpotSymbols, cfg.BinanceFuturesSymbols)
	store, err := storage.OpenPredictionStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("prediction store init failed: %v", err)
	}
	marketService := market.NewService(inference.New(cfg.InferenceURL, cfg.InferenceModel), provider, store, cfg)

	mux.HandleFunc("GET /health", handlers.Health(cfg))
	mux.HandleFunc("GET /api/markets/overview", handlers.MarketOverview(marketService))
	mux.HandleFunc("GET /api/statistics", handlers.AllStatistics(marketService))
	mux.HandleFunc("GET /api/markets/{market}/symbols/{symbol}/snapshot", handlers.SymbolSnapshot(marketService))
	mux.HandleFunc("GET /api/markets/{market}/symbols/{symbol}/depth", handlers.Depth(marketService))
	mux.HandleFunc("GET /api/markets/{market}/symbols/{symbol}/predictions/latest", handlers.LatestPrediction(marketService))
	mux.HandleFunc("GET /api/markets/{market}/symbols/{symbol}/predictions/history", handlers.PredictionHistory(marketService))
	mux.HandleFunc("GET /api/markets/{market}/symbols/{symbol}/statistics", handlers.SymbolStatistics(marketService))
	mux.HandleFunc("POST /api/auth/login", handlers.Login(cfg))
	mux.HandleFunc("GET /ws", handlers.Stream(hub))

	handler := middleware.SecurityHeaders(mux)
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)
	handler = middleware.RateLimit(cfg.RateLimitRPM)(handler)
	handler = middleware.RequestLogger(handler)

	return handler
}
