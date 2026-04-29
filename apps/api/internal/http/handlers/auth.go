package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pulsealpha/api/internal/config"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Login(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var input loginRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		email := strings.TrimSpace(strings.ToLower(input.Email))
		if email == "" || input.Password == "" {
			http.Error(w, "email and password are required", http.StatusBadRequest)
			return
		}

		accessToken := mintOpaqueToken(email + ":access")
		refreshToken := mintOpaqueToken(email + ":refresh")

		writeJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"email":         email,
				"role":          "user",
				"emailVerified": true,
			},
			"tokens": map[string]any{
				"accessToken":          accessToken,
				"accessTokenExpiresAt": time.Now().UTC().Add(cfg.AccessTokenTTL),
				"refreshToken":         refreshToken,
				"refreshTokenExpiresAt": time.Now().UTC().Add(cfg.RefreshTokenTTL),
				"rotationRequired":     true,
			},
		})
	}
}

func mintOpaqueToken(seed string) string {
	sum := sha256.Sum256([]byte(seed + time.Now().UTC().String()))
	return hex.EncodeToString(sum[:])
}
