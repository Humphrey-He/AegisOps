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
	if taskPayload.Data.Status != model.TaskStatusSuccess || len(taskPayload.Data.Steps) != 5 {
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

func TestPermissionsEndpointBackfillsMissingSeedPermissions(t *testing.T) {
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

	missingCodes := []string{"dashboard.view", "hosts.view", "audits.view", "users.view"}
	if err := database.Where("permission_id IN (?)",
		database.Model(&model.Permission{}).Select("id").Where("code IN ?", missingCodes),
	).Delete(&model.RolePermission{}).Error; err != nil {
		t.Fatalf("delete role permissions for missing codes: %v", err)
	}
	if err := database.Where("code IN ?", missingCodes).Delete(&model.Permission{}).Error; err != nil {
		t.Fatalf("delete missing seed permissions: %v", err)
	}

	resp := performRequest(router, http.MethodGet, "/api/permissions", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/permissions status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload struct {
		Data struct {
			Items []struct {
				Code string `json:"code"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode permissions response: %v; body=%s", err, resp.Body.String())
	}
	returned := make(map[string]bool, len(payload.Data.Items))
	for _, permission := range payload.Data.Items {
		returned[permission.Code] = true
	}
	for _, code := range missingCodes {
		if !returned[code] {
			t.Fatalf("GET /api/permissions did not backfill %q; returned=%v", code, returned)
		}
	}
}

func TestNotificationAlertAndHealthRoutesSmoke(t *testing.T) {
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

	createChannel := performRequest(router, http.MethodPost, "/api/notifications/channels", []byte(`{
		"name":"Ops Telegram",
		"type":"telegram",
		"enabled":true,
		"language":"en-US",
		"config":"{}",
		"defaultTarget":"ops"
	}`), token)
	if createChannel.Code != http.StatusCreated {
		t.Fatalf("POST /api/notifications/channels status = %d, want %d; body=%s", createChannel.Code, http.StatusCreated, createChannel.Body.String())
	}
	var channelPayload struct {
		Data struct {
			ID       string `json:"id"`
			Language string `json:"language"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createChannel.Body.Bytes(), &channelPayload); err != nil {
		t.Fatalf("decode notification channel: %v", err)
	}
	if channelPayload.Data.Language != "en-US" {
		t.Fatalf("notification channel language = %q, want en-US", channelPayload.Data.Language)
	}
	testChannel := performRequest(router, http.MethodPost, "/api/notifications/channels/"+channelPayload.Data.ID+"/test", nil, token)
	if testChannel.Code != http.StatusOK {
		t.Fatalf("POST /api/notifications/channels/:id/test status = %d, want %d; body=%s", testChannel.Code, http.StatusOK, testChannel.Body.String())
	}

	createRule := performRequest(router, http.MethodPost, "/api/alert-rules", []byte(`{
		"name":"Service health failed",
		"eventType":"service_health_check_failed",
		"resourceType":"service",
		"channelIds":"`+channelPayload.Data.ID+`",
		"language":"zh-CN",
		"enabled":true
	}`), token)
	if createRule.Code != http.StatusCreated {
		t.Fatalf("POST /api/alert-rules status = %d, want %d; body=%s", createRule.Code, http.StatusCreated, createRule.Body.String())
	}
	var rulePayload struct {
		Data struct {
			Language string `json:"language"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRule.Body.Bytes(), &rulePayload); err != nil {
		t.Fatalf("decode alert rule: %v", err)
	}
	if rulePayload.Data.Language != "zh-CN" {
		t.Fatalf("alert rule language = %q, want zh-CN", rulePayload.Data.Language)
	}

	createEvent := performRequest(router, http.MethodPost, "/api/alerts/events", []byte(`{
		"eventType":"service_health_check_failed",
		"resourceType":"service",
		"resourceId":"svc-smoke",
		"severity":"critical",
		"summary":"service smoke alert",
		"detail":"service smoke alert detail",
		"dedupeKey":"service_health_check_failed:svc-smoke"
	}`), token)
	if createEvent.Code != http.StatusCreated {
		t.Fatalf("POST /api/alerts/events status = %d, want %d; body=%s", createEvent.Code, http.StatusCreated, createEvent.Body.String())
	}

	for _, target := range []string{
		"/api/notifications/channels",
		"/api/notifications/records",
		"/api/alert-rules",
		"/api/alerts/events",
	} {
		resp := performRequest(router, http.MethodGet, target, nil, token)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d; body=%s", target, resp.Code, http.StatusOK, resp.Body.String())
		}
	}

	records := performRequest(router, http.MethodGet, "/api/notifications/records", nil, token)
	var recordsPayload struct {
		Data struct {
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(records.Body.Bytes(), &recordsPayload); err != nil {
		t.Fatalf("decode notification records: %v", err)
	}
	if recordsPayload.Data.Total == 0 {
		t.Fatalf("notification records total = 0, want alert event dispatch record")
	}
}

func TestExportBackupRoutesSmoke(t *testing.T) {
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

	createService := performRequest(router, http.MethodPost, "/api/services", []byte(`{
		"name":"Export Demo",
		"code":"export-demo",
		"image":"registry.local/export/demo",
		"defaultTag":"1.0.0"
	}`), token)
	if createService.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, want %d; body=%s", createService.Code, http.StatusCreated, createService.Body.String())
	}
	var servicePayload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createService.Body.Bytes(), &servicePayload); err != nil {
		t.Fatalf("decode service response: %v", err)
	}

	exportService := performRequest(router, http.MethodPost, "/api/exports/resources", []byte(`{
		"resourceType":"service",
		"resourceId":"`+servicePayload.Data.ID+`",
		"masked":true
	}`), token)
	if exportService.Code != http.StatusCreated {
		t.Fatalf("POST /api/exports/resources status = %d, want %d; body=%s", exportService.Code, http.StatusCreated, exportService.Body.String())
	}
	serviceExportID := decodeID(t, exportService.Body.Bytes())
	downloadService := performRequest(router, http.MethodGet, "/api/exports/"+serviceExportID+"/download", nil, token)
	if downloadService.Code != http.StatusOK {
		t.Fatalf("GET /api/exports/:id/download status = %d, want %d; body=%s", downloadService.Code, http.StatusOK, downloadService.Body.String())
	}

	createTask := performRequest(router, http.MethodPost, "/api/tasks", []byte(`{
		"type":"export.test",
		"title":"Export Incident Task",
		"targetType":"service",
		"targetId":"`+servicePayload.Data.ID+`",
		"steps":[{"name":"collect","sortOrder":1}]
	}`), token)
	if createTask.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks status = %d, want %d; body=%s", createTask.Code, http.StatusCreated, createTask.Body.String())
	}
	taskID := decodeID(t, createTask.Body.Bytes())
	incident := performRequest(router, http.MethodPost, "/api/exports/incidents", []byte(`{"taskId":"`+taskID+`","masked":true}`), token)
	if incident.Code != http.StatusCreated {
		t.Fatalf("POST /api/exports/incidents status = %d, want %d; body=%s", incident.Code, http.StatusCreated, incident.Body.String())
	}
	incidentID := decodeID(t, incident.Body.Bytes())
	downloadIncident := performRequest(router, http.MethodGet, "/api/exports/"+incidentID+"/download", nil, token)
	if downloadIncident.Code != http.StatusOK {
		t.Fatalf("GET /api/exports/:id/download incident status = %d, want %d; body=%s", downloadIncident.Code, http.StatusOK, downloadIncident.Body.String())
	}

	backup := performRequest(router, http.MethodPost, "/api/backups", []byte(`{"masked":true}`), token)
	if backup.Code != http.StatusCreated {
		t.Fatalf("POST /api/backups status = %d, want %d; body=%s", backup.Code, http.StatusCreated, backup.Body.String())
	}
	backupID := decodeID(t, backup.Body.Bytes())
	manifest := performRequest(router, http.MethodGet, "/api/backups/"+backupID+"/manifest", nil, token)
	if manifest.Code != http.StatusOK {
		t.Fatalf("GET /api/backups/:id/manifest status = %d, want %d; body=%s", manifest.Code, http.StatusOK, manifest.Body.String())
	}
	downloadBackup := performRequest(router, http.MethodGet, "/api/backups/"+backupID+"/download", nil, token)
	if downloadBackup.Code != http.StatusOK {
		t.Fatalf("GET /api/backups/:id/download status = %d, want %d; body=%s", downloadBackup.Code, http.StatusOK, downloadBackup.Body.String())
	}
}

func TestSecuritySchedulerRoutesSmoke(t *testing.T) {
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
		"username":"taskviewer",
		"password":"plain123456",
		"displayName":"Task Viewer",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	plainLogin := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"taskviewer","password":"plain123456"}`), "")
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
		t.Fatalf("decode login: %v", err)
	}
	forbiddenTaskCreate := performRequest(router, http.MethodPost, "/api/tasks", []byte(`{"type":"manual","title":"should fail"}`), loginPayload.Data.Tokens.AccessToken)
	if forbiddenTaskCreate.Code != http.StatusForbidden {
		t.Fatalf("POST /api/tasks without tasks.create status = %d, want %d; body=%s", forbiddenTaskCreate.Code, http.StatusForbidden, forbiddenTaskCreate.Body.String())
	}

	createSecret := performRequest(router, http.MethodPost, "/api/secrets", []byte(`{
		"name":"Telegram Bot Secret",
		"type":"API_TOKEN",
		"purpose":"telegram",
		"value":"{\"botToken\":\"token-value\",\"chatId\":\"7433377081\"}"
	}`), adminToken)
	if createSecret.Code != http.StatusCreated {
		t.Fatalf("POST /api/secrets status = %d, want %d; body=%s", createSecret.Code, http.StatusCreated, createSecret.Body.String())
	}
	secretID := decodeID(t, createSecret.Body.Bytes())

	createChannel := performRequest(router, http.MethodPost, "/api/notifications/channels", []byte(`{
		"name":"Secret Telegram",
		"type":"telegram",
		"enabled":true,
		"language":"zh-CN",
		"configSecretId":"`+secretID+`",
		"defaultTarget":"7433377081"
	}`), adminToken)
	if createChannel.Code != http.StatusCreated {
		t.Fatalf("POST /api/notifications/channels status = %d, want %d; body=%s", createChannel.Code, http.StatusCreated, createChannel.Body.String())
	}
	if bytes.Contains(createChannel.Body.Bytes(), []byte("token-value")) {
		t.Fatalf("notification channel response leaked secret config: %s", createChannel.Body.String())
	}
	refs := performRequest(router, http.MethodGet, "/api/secrets/"+secretID+"/references", nil, adminToken)
	if refs.Code != http.StatusOK {
		t.Fatalf("GET /api/secrets/:id/references status = %d, want %d; body=%s", refs.Code, http.StatusOK, refs.Body.String())
	}
	deleteSecret := performRequest(router, http.MethodDelete, "/api/secrets/"+secretID, nil, adminToken)
	if deleteSecret.Code != http.StatusBadRequest {
		t.Fatalf("DELETE referenced secret status = %d, want %d; body=%s", deleteSecret.Code, http.StatusBadRequest, deleteSecret.Body.String())
	}

	createJob := performRequest(router, http.MethodPost, "/api/scheduled-jobs", []byte(`{
		"name":"Host availability sweep",
		"type":"host.availability",
		"cronExpr":"*/5 * * * *",
		"targetType":"host",
		"targetId":"all",
		"timeoutSeconds":60
	}`), adminToken)
	if createJob.Code != http.StatusCreated {
		t.Fatalf("POST /api/scheduled-jobs status = %d, want %d; body=%s", createJob.Code, http.StatusCreated, createJob.Body.String())
	}
	jobID := decodeID(t, createJob.Body.Bytes())
	getJob := performRequest(router, http.MethodGet, "/api/scheduled-jobs/"+jobID, nil, adminToken)
	if getJob.Code != http.StatusOK {
		t.Fatalf("GET /api/scheduled-jobs/:id status = %d, want %d; body=%s", getJob.Code, http.StatusOK, getJob.Body.String())
	}
}

func TestNotificationLegacyConfigIsSecretized(t *testing.T) {
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
	createChannel := performRequest(router, http.MethodPost, "/api/notifications/channels", []byte(`{
		"name":"Legacy Telegram",
		"type":"telegram",
		"enabled":true,
		"config":"{\"botToken\":\"legacy-token\",\"chatId\":\"7433377081\"}",
		"defaultTarget":"7433377081"
	}`), token)
	if createChannel.Code != http.StatusCreated {
		t.Fatalf("POST /api/notifications/channels status = %d, want %d; body=%s", createChannel.Code, http.StatusCreated, createChannel.Body.String())
	}
	if bytes.Contains(createChannel.Body.Bytes(), []byte("legacy-token")) {
		t.Fatalf("legacy config leaked in response: %s", createChannel.Body.String())
	}
	channelID := decodeID(t, createChannel.Body.Bytes())
	var channel model.NotificationChannel
	if err := database.First(&channel, "id = ?", channelID).Error; err != nil {
		t.Fatalf("load notification channel: %v", err)
	}
	if channel.ConfigSecretID == "" {
		t.Fatalf("legacy config was not converted to configSecretId")
	}
	if channel.ConfigEncrypted != "" {
		t.Fatalf("legacy config remained in ConfigEncrypted: %q", channel.ConfigEncrypted)
	}
}

func TestHighRiskRoutesRequireDedicatedPermissions(t *testing.T) {
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
		"username":"limited",
		"password":"plain123456",
		"displayName":"Limited User",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	plainLogin := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"limited","password":"plain123456"}`), "")
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
		t.Fatalf("decode login: %v", err)
	}
	for _, target := range []string{
		"/api/hosts/host-1/test-ssh",
		"/api/docker/nodes/docker-1/test",
		"/api/registries/registry-1/test",
		"/api/nginx/nodes/nginx-1/reload",
	} {
		resp := performRequest(router, http.MethodPost, target, nil, loginPayload.Data.Tokens.AccessToken)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("POST %s status = %d, want %d; body=%s", target, resp.Code, http.StatusForbidden, resp.Body.String())
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

func decodeID(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode id response: %v; body=%s", err, string(body))
	}
	if payload.Data.ID == "" {
		t.Fatalf("response did not include id; body=%s", string(body))
	}
	return payload.Data.ID
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
