package config

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr              string
	AllowedOrigins        []string
	JWTIssuer             string
	JWTAudience           string
	AccessTokenTTL        time.Duration
	RefreshTokenTTL       time.Duration
	RateLimitRPM          int
	InferenceURL          string
	InferenceModel        string
	DBPath                string
	BinanceSpotURL        string
	BinanceFuturesURL     string
	BinanceSpotSymbols    []string
	BinanceFuturesSymbols []string
	MinConfidence         float64
	MaxSpreadBps          float64
	MinConsensusSamples   int
	MinDepthImbalance     float64
	MinMicroPriceBias     float64
}

func Load() Config {
	return Config{
		HTTPAddr:              getEnv("PULSEALPHA_HTTP_ADDR", ":8080"),
		AllowedOrigins:        splitCSV(getEnv("PULSEALPHA_ALLOWED_ORIGINS", "http://localhost:5173")),
		JWTIssuer:             getEnv("PULSEALPHA_JWT_ISSUER", "pulsealpha"),
		JWTAudience:           getEnv("PULSEALPHA_JWT_AUDIENCE", "pulsealpha-web"),
		AccessTokenTTL:        getDurationEnv("PULSEALPHA_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:       getDurationEnv("PULSEALPHA_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		RateLimitRPM:          getIntEnv("PULSEALPHA_RATE_LIMIT_RPM", 120),
		InferenceURL:          getEnv("PULSEALPHA_INFERENCE_URL", ""),
		InferenceModel:        getEnv("PULSEALPHA_INFERENCE_MODEL", "deeplob-freqai-ensemble-1m-v2"),
		DBPath:                getEnv("PULSEALPHA_DB_PATH", "./pulsealpha.db"),
		BinanceSpotURL:        getEnv("PULSEALPHA_BINANCE_SPOT_URL", "https://api.binance.com"),
		BinanceFuturesURL:     getEnv("PULSEALPHA_BINANCE_FUTURES_URL", "https://fapi.binance.com"),
		BinanceSpotSymbols:    splitCSV(getEnv("PULSEALPHA_BINANCE_SPOT_SYMBOLS", "BTCUSDT,ETHUSDT,SOLUSDT")),
		BinanceFuturesSymbols: splitCSV(getEnv("PULSEALPHA_BINANCE_FUTURES_SYMBOLS", "BTCUSDT,ETHUSDT,SOLUSDT")),
		MinConfidence:         getFloatEnv("PULSEALPHA_MIN_CONFIDENCE", 0.58),
		MaxSpreadBps:          getFloatEnv("PULSEALPHA_MAX_SPREAD_BPS", 12),
		MinConsensusSamples:   getIntEnv("PULSEALPHA_MIN_CONSENSUS_SAMPLES", 5),
		MinDepthImbalance:     getFloatEnv("PULSEALPHA_MIN_DEPTH_IMBALANCE", 0.03),
		MinMicroPriceBias:     getFloatEnv("PULSEALPHA_MIN_MICROPRICE_BIAS", 0.008),
	}
}

func (c Config) ShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getFloatEnv(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
