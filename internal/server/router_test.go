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
	"github.com/Humphrey-He/AegisOps/internal/model"
	"go.uber.org/zap"
)

func TestSmokeHealthLoginAndMe(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)

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

	token := loginAndToken(t, router)

	meResp := performRequest(router, http.MethodGet, "/api/auth/me", nil, token)
	if meResp.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/me status = %d, want %d; body=%s", meResp.Code, http.StatusOK, meResp.Body.String())
	}
}

func TestRegistryCRUDSmoke(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)

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
	token := loginAndToken(t, router)

	createResp := performRequest(router, http.MethodPost, "/api/registries", []byte(`{
		"name":"Local Registry",
		"url":"http://registry.local:5000",
		"authType":"NONE",
		"description":"smoke registry"
	}`), token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/registries status = %d, want %d; body=%s", createResp.Code, http.StatusCreated, createResp.Body.String())
	}
	var createPayload struct {
		Data struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create registry response: %v; body=%s", err, createResp.Body.String())
	}
	if createPayload.Data.ID == "" || createPayload.Data.URL != "http://registry.local:5000" {
		t.Fatalf("unexpected registry create payload: %+v", createPayload.Data)
	}

	listResp := performRequest(router, http.MethodGet, "/api/registries", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("GET /api/registries status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}

	getResp := performRequest(router, http.MethodGet, "/api/registries/"+createPayload.Data.ID, nil, token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET /api/registries/:id status = %d, want %d; body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
	}

	updateResp := performRequest(router, http.MethodPatch, "/api/registries/"+createPayload.Data.ID, []byte(`{
		"name":"Local Registry Updated",
		"description":"updated"
	}`), token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("PATCH /api/registries/:id status = %d, want %d; body=%s", updateResp.Code, http.StatusOK, updateResp.Body.String())
	}

	deleteResp := performRequest(router, http.MethodDelete, "/api/registries/"+createPayload.Data.ID, nil, token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/registries/:id status = %d, want %d; body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}
}

func TestRegistryDeleteRejectsServiceReference(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	database, err := db.Open(cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	router := NewRouter(cfg, database, zap.NewNop())
	token := loginAndToken(t, router)
	createRegistry := performRequest(router, http.MethodPost, "/api/registries", []byte(`{"name":"Ref Registry","url":"http://registry.local:5000","authType":"NONE"}`), token)
	if createRegistry.Code != http.StatusCreated {
		t.Fatalf("POST /api/registries status = %d, want %d; body=%s", createRegistry.Code, http.StatusCreated, createRegistry.Body.String())
	}
	var registryPayload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRegistry.Body.Bytes(), &registryPayload); err != nil {
		t.Fatalf("decode registry response: %v", err)
	}
	createService := performRequest(router, http.MethodPost, "/api/services", []byte(`{
		"name":"Ref Service",
		"code":"ref-service",
		"registryId":"`+registryPayload.Data.ID+`",
		"image":"registry.local/ref/service"
	}`), token)
	if createService.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, want %d; body=%s", createService.Code, http.StatusCreated, createService.Body.String())
	}
	deleteRegistry := performRequest(router, http.MethodDelete, "/api/registries/"+registryPayload.Data.ID, nil, token)
	if deleteRegistry.Code != http.StatusBadRequest {
		t.Fatalf("DELETE /api/registries/:id status = %d, want %d; body=%s", deleteRegistry.Code, http.StatusBadRequest, deleteRegistry.Body.String())
	}
}

func TestServiceReleaseSmoke(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)

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
	token := loginAndToken(t, router)

	createResp := performRequest(router, http.MethodPost, "/api/services", []byte(`{
		"name":"Demo API",
		"code":"demo-api",
		"image":"registry.local/demo/api",
		"defaultTag":"1.0.0",
		"ports":[{"containerPort":8080,"hostPort":18080}],
		"envs":[{"name":"APP_ENV","value":"dev"}],
		"targetId":"docker-node-1"
	}`), token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, want %d; body=%s", createResp.Code, http.StatusCreated, createResp.Body.String())
	}
	var createPayload struct {
		Data struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create service response: %v; body=%s", err, createResp.Body.String())
	}
	if createPayload.Data.ID == "" || createPayload.Data.Code != "demo-api" {
		t.Fatalf("unexpected service create payload: %+v", createPayload.Data)
	}

	releaseResp := performRequest(router, http.MethodPost, "/api/services/"+createPayload.Data.ID+"/releases", []byte(`{
		"imageTag":"1.0.1",
		"version":"2026.05.14"
	}`), token)
	if releaseResp.Code != http.StatusCreated {
		t.Fatalf("POST /api/services/:id/releases status = %d, want %d; body=%s", releaseResp.Code, http.StatusCreated, releaseResp.Body.String())
	}
	var releasePayload struct {
		Data struct {
			TaskID    string `json:"taskId"`
			ReleaseID string `json:"releaseId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(releaseResp.Body.Bytes(), &releasePayload); err != nil {
		t.Fatalf("decode release response: %v; body=%s", err, releaseResp.Body.String())
	}
	if releasePayload.Data.TaskID == "" || releasePayload.Data.ReleaseID == "" {
		t.Fatalf("unexpected release payload: %+v", releasePayload.Data)
	}

	for _, target := range []string{
		"/api/services/" + createPayload.Data.ID + "/instances",
		"/api/services/" + createPayload.Data.ID + "/versions",
		"/api/services/" + createPayload.Data.ID + "/releases",
		"/api/tasks/" + releasePayload.Data.TaskID,
	} {
		resp := performRequest(router, http.MethodGet, target, nil, token)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d; body=%s", target, resp.Code, http.StatusOK, resp.Body.String())
		}
	}

	taskResp := performRequest(router, http.MethodGet, "/api/tasks/"+releasePayload.Data.TaskID, nil, token)
	var taskPayload struct {
		Data model.Task `json:"data"`
	}
	if err := json.Unmarshal(taskResp.Body.Bytes(), &taskPayload); err != nil {
		t.Fatalf("decode task response: %v; body=%s", err, taskResp.Body.String())
	}
	if taskPayload.Data.Status != model.TaskStatusSuccess || len(taskPayload.Data.Steps) != 4 {
		t.Fatalf("unexpected release task: status=%s steps=%d", taskPayload.Data.Status, len(taskPayload.Data.Steps))
	}
	for _, step := range taskPayload.Data.Steps {
		if step.Status != model.TaskStatusSuccess {
			t.Fatalf("release step %q status=%s, want %s", step.Name, step.Status, model.TaskStatusSuccess)
		}
	}
}

func TestSystemRoutesRequireRBAC(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	database, err := db.Open(cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	router := NewRouter(cfg, database, zap.NewNop())
	adminToken := loginAndToken(t, router)
	createUser := performRequest(router, http.MethodPost, "/api/users", []byte(`{
		"username":"plain",
		"password":"plain123456",
		"displayName":"Plain User",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	plainLogin := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"plain","password":"plain123456"}`), "")
	if plainLogin.Code != http.StatusOK {
		t.Fatalf("plain login status = %d, want %d; body=%s", plainLogin.Code, http.StatusOK, plainLogin.Body.String())
	}
	var loginPayload struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"accessToken"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(plainLogin.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode plain login: %v", err)
	}
	for _, target := range []string{"/api/users", "/api/roles", "/api/permissions", "/api/audits"} {
		resp := performRequest(router, http.MethodGet, target, nil, loginPayload.Data.Tokens.AccessToken)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("GET %s status = %d, want %d; body=%s", target, resp.Code, http.StatusForbidden, resp.Body.String())
		}
	}
}

func TestTerminalWebSocketRouteRequiresAuthAndSession(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	database, err := db.Open(cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	router := NewRouter(cfg, database, zap.NewNop())
	unauthorized := performRequest(router, http.MethodGet, "/api/terminal/sessions/missing/ws", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/terminal/sessions/:id/ws without token status = %d, want %d; body=%s", unauthorized.Code, http.StatusUnauthorized, unauthorized.Body.String())
	}

	token := loginAndToken(t, router)
	missing := performWebSocketRequest(router, "/api/terminal/sessions/missing/ws", token)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET /api/terminal/sessions/:id/ws missing status = %d, want %d; body=%s", missing.Code, http.StatusNotFound, missing.Body.String())
	}
}

func TestTerminalWebSocketRouteRequiresUpgradeForExistingSession(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	database, err := db.Open(cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	session := model.TerminalSession{
		ID:       "terminal-session-1",
		HostID:   "host-1",
		HostName: "prod-host",
		Status:   model.TerminalSessionStatusConnected,
	}
	if err := database.Create(&session).Error; err != nil {
		t.Fatalf("seed terminal session: %v", err)
	}

	router := NewRouter(cfg, database, zap.NewNop())
	token := loginAndToken(t, router)
	resp := performRequest(router, http.MethodGet, "/api/terminal/sessions/terminal-session-1/ws", nil, token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/terminal/sessions/:id/ws without upgrade status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
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
}

func loginAndToken(t *testing.T, router http.Handler) string {
	t.Helper()
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
	return loginPayload.Data.Tokens.AccessToken
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

func performWebSocketRequest(handler http.Handler, target string, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
