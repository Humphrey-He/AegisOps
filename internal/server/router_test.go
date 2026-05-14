package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"go.uber.org/zap"
)

func TestSmokeHealthLoginAndMe(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		App: config.AppConfig{
			Name: "aegisops",
			Env:  "test",
		},
		HTTP: config.HTTPConfig{
			Addr: ":0",
		},
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
		},
		Security: config.SecurityConfig{
			JWTSecret:       "test-jwt-secret",
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
			SecretKey:       "test-secret-key",
		},
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "admin123456",
			Email:    "admin@example.com",
		},
	}

	database, err := db.Open(cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	router := NewRouter(cfg, database, zap.NewNop())

	healthResp := performRequest(router, http.MethodGet, "/healthz", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d; body=%s", healthResp.Code, http.StatusOK, healthResp.Body.String())
	}

	loginBody := []byte(`{"username":"admin","password":"admin123456"}`)
	loginResp := performRequest(router, http.MethodPost, "/api/auth/login", loginBody, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("POST /api/auth/login status = %d, want %d; body=%s", loginResp.Code, http.StatusOK, loginResp.Body.String())
	}

	var loginPayload struct {
		Code string `json:"code"`
		Data struct {
			User struct {
				Username string `json:"username"`
				IsAdmin  bool   `json:"isAdmin"`
			} `json:"user"`
			Tokens struct {
				AccessToken string `json:"accessToken"`
				TokenType   string `json:"tokenType"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login response: %v; body=%s", err, loginResp.Body.String())
	}
	if loginPayload.Code != "OK" || loginPayload.Data.User.Username != "admin" || !loginPayload.Data.User.IsAdmin {
		t.Fatalf("unexpected login payload: %+v", loginPayload)
	}
	if loginPayload.Data.Tokens.AccessToken == "" {
		t.Fatal("login response did not include access token")
	}

	meResp := performRequest(router, http.MethodGet, "/api/auth/me", nil, loginPayload.Data.Tokens.AccessToken)
	if meResp.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/me status = %d, want %d; body=%s", meResp.Code, http.StatusOK, meResp.Body.String())
	}
}

func performRequest(handler http.Handler, method string, target string, body []byte, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
