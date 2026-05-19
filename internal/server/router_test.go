package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	schedulersvc "github.com/Humphrey-He/AegisOps/internal/scheduler"
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

func TestEnvironmentRoutesFilterResourcesAndProtectDelete(t *testing.T) {
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

	createEnv := performRequest(router, http.MethodPost, "/api/environments", []byte(`{"name":"Production","code":"prod","description":"production env","sortOrder":1}`), token)
	if createEnv.Code != http.StatusCreated {
		t.Fatalf("POST /api/environments status = %d, want %d; body=%s", createEnv.Code, http.StatusCreated, createEnv.Body.String())
	}
	var envPayload struct {
		Data struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createEnv.Body.Bytes(), &envPayload); err != nil {
		t.Fatalf("decode environment response: %v", err)
	}
	if envPayload.Data.Code != "prod" {
		t.Fatalf("environment code = %q, want prod", envPayload.Data.Code)
	}
	missingEnvDocker := performRequest(router, http.MethodPost, "/api/docker/nodes", []byte(`{"name":"missing-env-docker","endpoint":"mock://missing","authType":"NONE","environment":"missing"}`), token)
	if missingEnvDocker.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/docker/nodes missing environment status = %d, want %d; body=%s", missingEnvDocker.Code, http.StatusBadRequest, missingEnvDocker.Body.String())
	}
	disabledEnv := performRequest(router, http.MethodPost, "/api/environments", []byte(`{"name":"Disabled","code":"disabled","status":"DISABLED"}`), token)
	if disabledEnv.Code != http.StatusCreated {
		t.Fatalf("POST disabled environment status = %d, want %d; body=%s", disabledEnv.Code, http.StatusCreated, disabledEnv.Body.String())
	}
	disabledEnvDocker := performRequest(router, http.MethodPost, "/api/docker/nodes", []byte(`{"name":"disabled-env-docker","endpoint":"mock://disabled","authType":"NONE","environment":"disabled"}`), token)
	if disabledEnvDocker.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/docker/nodes disabled environment status = %d, want %d; body=%s", disabledEnvDocker.Code, http.StatusBadRequest, disabledEnvDocker.Body.String())
	}

	prodDocker := performRequest(router, http.MethodPost, "/api/docker/nodes", []byte(`{"name":"prod-docker","endpoint":"mock://prod-docker","authType":"NONE","environment":"prod"}`), token)
	if prodDocker.Code != http.StatusCreated {
		t.Fatalf("POST /api/docker/nodes prod status = %d, want %d; body=%s", prodDocker.Code, http.StatusCreated, prodDocker.Body.String())
	}
	devDocker := performRequest(router, http.MethodPost, "/api/docker/nodes", []byte(`{"name":"dev-docker","endpoint":"mock://dev-docker","authType":"NONE","environment":"dev"}`), token)
	if devDocker.Code != http.StatusCreated {
		t.Fatalf("POST /api/docker/nodes dev status = %d, want %d; body=%s", devDocker.Code, http.StatusCreated, devDocker.Body.String())
	}

	listDocker := performRequest(router, http.MethodGet, "/api/docker/nodes?environment=prod", nil, token)
	if listDocker.Code != http.StatusOK {
		t.Fatalf("GET /api/docker/nodes status = %d, want %d; body=%s", listDocker.Code, http.StatusOK, listDocker.Body.String())
	}
	var dockerPayload struct {
		Data struct {
			Items []struct {
				Name        string `json:"name"`
				Environment string `json:"environment"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listDocker.Body.Bytes(), &dockerPayload); err != nil {
		t.Fatalf("decode docker list response: %v; body=%s", err, listDocker.Body.String())
	}
	if dockerPayload.Data.Total != 1 || len(dockerPayload.Data.Items) != 1 || dockerPayload.Data.Items[0].Name != "prod-docker" {
		t.Fatalf("unexpected prod docker list: %+v", dockerPayload.Data)
	}

	createService := performRequest(router, http.MethodPost, "/api/services", []byte(`{
		"name":"Prod Service",
		"code":"prod-service",
		"environment":"prod",
		"image":"nginx",
		"targetId":"`+decodeID(t, prodDocker.Body.Bytes())+`"
	}`), token)
	if createService.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, want %d; body=%s", createService.Code, http.StatusCreated, createService.Body.String())
	}

	listServices := performRequest(router, http.MethodGet, "/api/services?environment=prod", nil, token)
	if listServices.Code != http.StatusOK {
		t.Fatalf("GET /api/services status = %d, want %d; body=%s", listServices.Code, http.StatusOK, listServices.Body.String())
	}

	deleteEnv := performRequest(router, http.MethodDelete, "/api/environments/"+envPayload.Data.ID, nil, token)
	if deleteEnv.Code != http.StatusBadRequest {
		t.Fatalf("DELETE referenced environment status = %d, want %d; body=%s", deleteEnv.Code, http.StatusBadRequest, deleteEnv.Body.String())
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

func TestResourceWorkbenchFiltersTasksAndAudits(t *testing.T) {
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

	createHostTask := performRequest(router, http.MethodPost, "/api/tasks", []byte(`{
		"type":"host.ssh.test",
		"title":"host task",
		"targetType":"host",
		"targetId":"host-workbench-1"
	}`), token)
	if createHostTask.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks host status = %d, want %d; body=%s", createHostTask.Code, http.StatusCreated, createHostTask.Body.String())
	}
	createDockerTask := performRequest(router, http.MethodPost, "/api/tasks", []byte(`{
		"type":"docker.node.test",
		"title":"docker task",
		"targetType":"docker_node",
		"targetId":"docker-workbench-1"
	}`), token)
	if createDockerTask.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks docker status = %d, want %d; body=%s", createDockerTask.Code, http.StatusCreated, createDockerTask.Body.String())
	}

	if err := database.Create(&model.AuditLog{Username: "admin", Action: "host.test_ssh", ResourceType: "host", ResourceID: "host-workbench-1", Result: model.AuditResultSuccess}).Error; err != nil {
		t.Fatalf("create host audit: %v", err)
	}
	if err := database.Create(&model.AuditLog{Username: "admin", Action: "docker_node.test", ResourceType: "docker_node", ResourceID: "docker-workbench-1", Result: model.AuditResultSuccess}).Error; err != nil {
		t.Fatalf("create docker audit: %v", err)
	}

	tasksResp := performRequest(router, http.MethodGet, "/api/tasks?resourceType=host&resourceId=host-workbench-1", nil, token)
	if tasksResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks resource filter status = %d, want %d; body=%s", tasksResp.Code, http.StatusOK, tasksResp.Body.String())
	}
	var tasksPayload struct {
		Data struct {
			Items []model.Task `json:"items"`
			Total int64        `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(tasksResp.Body.Bytes(), &tasksPayload); err != nil {
		t.Fatalf("decode tasks payload: %v", err)
	}
	if tasksPayload.Data.Total != 1 || len(tasksPayload.Data.Items) != 1 || tasksPayload.Data.Items[0].TargetID != "host-workbench-1" {
		t.Fatalf("filtered tasks = %+v total=%d, want only host-workbench-1", tasksPayload.Data.Items, tasksPayload.Data.Total)
	}
	tasksAliasResp := performRequest(router, http.MethodGet, "/api/tasks?targetType=docker_node&targetId=docker-workbench-1", nil, token)
	if tasksAliasResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks target filter status = %d, want %d; body=%s", tasksAliasResp.Code, http.StatusOK, tasksAliasResp.Body.String())
	}
	var tasksAliasPayload struct {
		Data struct {
			Items []model.Task `json:"items"`
			Total int64        `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(tasksAliasResp.Body.Bytes(), &tasksAliasPayload); err != nil {
		t.Fatalf("decode tasks alias payload: %v", err)
	}
	if tasksAliasPayload.Data.Total != 1 || len(tasksAliasPayload.Data.Items) != 1 || tasksAliasPayload.Data.Items[0].TargetID != "docker-workbench-1" {
		t.Fatalf("filtered alias tasks = %+v total=%d, want only docker-workbench-1", tasksAliasPayload.Data.Items, tasksAliasPayload.Data.Total)
	}

	auditsResp := performRequest(router, http.MethodGet, "/api/audits?resourceType=host&resourceId=host-workbench-1", nil, token)
	if auditsResp.Code != http.StatusOK {
		t.Fatalf("GET /api/audits resource filter status = %d, want %d; body=%s", auditsResp.Code, http.StatusOK, auditsResp.Body.String())
	}
	var auditsPayload struct {
		Data struct {
			Items []model.AuditLog `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(auditsResp.Body.Bytes(), &auditsPayload); err != nil {
		t.Fatalf("decode audits payload: %v", err)
	}
	if auditsPayload.Data.Total != 1 || len(auditsPayload.Data.Items) != 1 || auditsPayload.Data.Items[0].ResourceID != "host-workbench-1" {
		t.Fatalf("filtered audits = %+v total=%d, want only host-workbench-1", auditsPayload.Data.Items, auditsPayload.Data.Total)
	}
}

func TestDashboardRiskAndResourceContext(t *testing.T) {
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

	host := model.Host{ID: "risk-host-1", Name: "Risk Host", Address: "10.0.0.8", SSHPort: 22, SSHUser: "root", SSHSecretID: "secret-1", Status: model.HostStatusOffline}
	if err := database.Create(&host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	task := model.Task{ID: "risk-task-1", Type: "host.ssh.test", Title: "risk host ssh", Status: model.TaskStatusFailed, TargetType: "host", TargetID: host.ID, Error: "connect timeout"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	alert := model.AlertEvent{ID: "risk-alert-1", EventType: "host_unreachable", ResourceType: "host", ResourceID: host.ID, TaskID: task.ID, Severity: model.AlertEventSeverityCritical, Status: model.AlertEventStatusOpen, Summary: "host unreachable", FirstTriggeredAt: time.Now().UTC(), LastTriggeredAt: time.Now().UTC()}
	if err := database.Create(&alert).Error; err != nil {
		t.Fatalf("create alert: %v", err)
	}
	auditLog := model.AuditLog{Username: "admin", Action: "secret.delete", ResourceType: "host", ResourceID: host.ID, Result: model.AuditResultSuccess}
	if err := database.Create(&auditLog).Error; err != nil {
		t.Fatalf("create audit: %v", err)
	}

	summaryResp := performRequest(router, http.MethodGet, "/api/dashboard/summary", nil, token)
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("GET /api/dashboard/summary status = %d, want %d; body=%s", summaryResp.Code, http.StatusOK, summaryResp.Body.String())
	}
	var summaryPayload struct {
		Data struct {
			OpenAlertCount         int64 `json:"openAlertCount"`
			FailedTaskCount        int64 `json:"failedTaskCount"`
			HighRiskAuditCount     int64 `json:"highRiskAuditCount"`
			UnhealthyResourceCount int64 `json:"unhealthyResourceCount"`
			OpenAlerts             []model.AlertEvent
			FailedTasks            []model.Task     `json:"failedTasks"`
			UnhealthyResources     []map[string]any `json:"unhealthyResources"`
			HighRiskAudits         []model.AuditLog `json:"highRiskAudits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(summaryResp.Body.Bytes(), &summaryPayload); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summaryPayload.Data.OpenAlertCount == 0 || summaryPayload.Data.FailedTaskCount == 0 || summaryPayload.Data.HighRiskAuditCount == 0 || summaryPayload.Data.UnhealthyResourceCount == 0 {
		t.Fatalf("summary counts missing risk data: %+v", summaryPayload.Data)
	}
	if len(summaryPayload.Data.OpenAlerts) == 0 || len(summaryPayload.Data.FailedTasks) == 0 || len(summaryPayload.Data.UnhealthyResources) == 0 || len(summaryPayload.Data.HighRiskAudits) == 0 {
		t.Fatalf("summary risk lists missing data: %+v", summaryPayload.Data)
	}

	contextResp := performRequest(router, http.MethodGet, "/api/resources/context?resourceType=host&resourceId="+host.ID, nil, token)
	if contextResp.Code != http.StatusOK {
		t.Fatalf("GET /api/resources/context status = %d, want %d; body=%s", contextResp.Code, http.StatusOK, contextResp.Body.String())
	}
	var contextPayload struct {
		Data struct {
			ResourceType string `json:"resourceType"`
			ResourceID   string `json:"resourceId"`
			Navigation   struct {
				DetailPath string `json:"detailPath"`
				TasksPath  string `json:"tasksPath"`
				AuditsPath string `json:"auditsPath"`
				AlertsPath string `json:"alertsPath"`
			} `json:"navigation"`
			PrimaryAction struct {
				Key        string `json:"key"`
				Label      string `json:"label"`
				Permission string `json:"permission"`
			} `json:"primaryAction"`
			Risk struct {
				Level             string `json:"level"`
				OpenAlertCount    int64  `json:"openAlertCount"`
				FailedTaskCount   int64  `json:"failedTaskCount"`
				LastFailureReason string `json:"lastFailureReason"`
			} `json:"risk"`
			RecentTasks  []model.Task       `json:"recentTasks"`
			RecentAudits []model.AuditLog   `json:"recentAudits"`
			RecentAlerts []model.AlertEvent `json:"recentAlerts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(contextResp.Body.Bytes(), &contextPayload); err != nil {
		t.Fatalf("decode resource context: %v", err)
	}
	if contextPayload.Data.ResourceType != "host" || contextPayload.Data.ResourceID != host.ID {
		t.Fatalf("resource context identity = %s/%s, want host/%s", contextPayload.Data.ResourceType, contextPayload.Data.ResourceID, host.ID)
	}
	if len(contextPayload.Data.RecentTasks) != 1 || len(contextPayload.Data.RecentAudits) != 1 || len(contextPayload.Data.RecentAlerts) != 1 {
		t.Fatalf("resource context lists = tasks:%d audits:%d alerts:%d, want 1 each", len(contextPayload.Data.RecentTasks), len(contextPayload.Data.RecentAudits), len(contextPayload.Data.RecentAlerts))
	}
	if contextPayload.Data.Navigation.DetailPath != "/assets/hosts?selected="+host.ID {
		t.Fatalf("resource detail path = %q, want selected host path", contextPayload.Data.Navigation.DetailPath)
	}
	if contextPayload.Data.Navigation.TasksPath == "" || contextPayload.Data.Navigation.AuditsPath == "" || contextPayload.Data.Navigation.AlertsPath == "" {
		t.Fatalf("resource navigation paths incomplete: %+v", contextPayload.Data.Navigation)
	}
	if contextPayload.Data.PrimaryAction.Key != "test_ssh" || contextPayload.Data.PrimaryAction.Permission != "hosts.test" {
		t.Fatalf("primary action = %+v, want host SSH test", contextPayload.Data.PrimaryAction)
	}
	if contextPayload.Data.Risk.Level != "critical" || contextPayload.Data.Risk.OpenAlertCount != 1 || contextPayload.Data.Risk.FailedTaskCount != 1 || contextPayload.Data.Risk.LastFailureReason == "" {
		t.Fatalf("risk summary = %+v, want critical resource risk", contextPayload.Data.Risk)
	}

	if err := database.Create(&model.DockerNode{ID: "risk-docker-1", Name: "Risk Docker", Endpoint: "tcp://127.0.0.1:2375", Status: model.DockerNodeStatusOnline}).Error; err != nil {
		t.Fatalf("create docker node: %v", err)
	}
	if err := database.Create(&model.NginxNode{ID: "risk-nginx-1", Name: "Risk Nginx", HostID: host.ID, ConfigPath: "/etc/nginx/nginx.conf", TestCommand: "nginx -t", ReloadCommand: "nginx -s reload", Status: model.NginxNodeStatusOnline}).Error; err != nil {
		t.Fatalf("create nginx node: %v", err)
	}
	if err := database.Create(&model.Registry{ID: "risk-registry-1", Name: "Risk Registry", URL: "http://registry.local:5000", AuthType: model.RegistryAuthTypeNone, Status: model.RegistryStatusOnline}).Error; err != nil {
		t.Fatalf("create registry: %v", err)
	}
	if err := database.Create(&model.ServiceDefinition{ID: "risk-service-1", Name: "Risk Service", Code: "risk-service", Image: "registry.local/risk/service", Status: model.ServiceStatusActive}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	searchResp := performRequest(router, http.MethodGet, "/api/resources/search?keyword=Risk&limit=10", nil, token)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("GET /api/resources/search status = %d, want %d; body=%s", searchResp.Code, http.StatusOK, searchResp.Body.String())
	}
	var searchPayload struct {
		Data struct {
			Items []struct {
				ResourceType string `json:"resourceType"`
				ResourceID   string `json:"resourceId"`
				Name         string `json:"name"`
				Path         string `json:"path"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(searchResp.Body.Bytes(), &searchPayload); err != nil {
		t.Fatalf("decode resource search: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range searchPayload.Data.Items {
		if item.Path == "" {
			t.Fatalf("search item missing path: %+v", item)
		}
		seen[item.ResourceType] = true
	}
	for _, resourceType := range []string{"host", "docker_node", "nginx_node", "registry", "service"} {
		if !seen[resourceType] {
			t.Fatalf("search results missing %s: %+v", resourceType, searchPayload.Data.Items)
		}
	}
}

func TestTaskAndAlertContextCloseTroubleshootingLoop(t *testing.T) {
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

	host := model.Host{ID: "context-host-1", Name: "Context Host", Address: "10.0.0.9", SSHPort: 22, SSHUser: "root", SSHSecretID: "secret-1", Status: model.HostStatusOffline}
	if err := database.Create(&host).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	taskItem := model.Task{ID: "context-task-1", Type: "host.ssh.test", Title: "context host ssh", Status: model.TaskStatusFailed, TargetType: "host", TargetID: host.ID, Error: "dial tcp timeout"}
	if err := database.Create(&taskItem).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := database.Create(&model.TaskStep{ID: "context-step-1", TaskID: taskItem.ID, Name: "connect host", Status: model.TaskStatusFailed, SortOrder: 1, Error: "ssh timeout"}).Error; err != nil {
		t.Fatalf("create task step: %v", err)
	}
	if err := database.Create(&model.TaskLog{ID: "context-log-1", TaskID: taskItem.ID, StepID: "context-step-1", Level: model.TaskLogLevelError, Message: "network unreachable"}).Error; err != nil {
		t.Fatalf("create task log: %v", err)
	}
	alertEvent := model.AlertEvent{ID: "context-alert-1", EventType: "host_offline", ResourceType: "host", ResourceID: host.ID, TaskID: taskItem.ID, Severity: model.AlertEventSeverityCritical, Status: model.AlertEventStatusOpen, Summary: "host offline", FirstTriggeredAt: time.Now().UTC(), LastTriggeredAt: time.Now().UTC()}
	if err := database.Create(&alertEvent).Error; err != nil {
		t.Fatalf("create alert: %v", err)
	}
	if err := database.Create(&model.AuditLog{Username: "admin", Action: "host.test_ssh", ResourceType: "host", ResourceID: host.ID, Result: model.AuditResultFailure, Message: "ssh failed"}).Error; err != nil {
		t.Fatalf("create audit: %v", err)
	}
	if err := database.Create(&model.NotificationRecord{ID: "context-notification-1", EventID: alertEvent.ID, ChannelID: "channel-1", ChannelName: "Telegram", ChannelType: model.NotificationChannelTypeTelegram, Status: model.NotificationRecordStatusSuccess}).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	taskContextResp := performRequest(router, http.MethodGet, "/api/tasks/"+taskItem.ID+"/context", nil, token)
	if taskContextResp.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks/:id/context status = %d, want %d; body=%s", taskContextResp.Code, http.StatusOK, taskContextResp.Body.String())
	}
	var taskPayload struct {
		Data struct {
			FailureSummary string                     `json:"failureSummary"`
			Resource       *map[string]any            `json:"resource"`
			RelatedAudits  []model.AuditLog           `json:"relatedAudits"`
			RelatedAlerts  []model.AlertEvent         `json:"relatedAlerts"`
			Notifications  []model.NotificationRecord `json:"notifications"`
			NextActions    []struct {
				Key string `json:"key"`
			} `json:"nextActions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(taskContextResp.Body.Bytes(), &taskPayload); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	if taskPayload.Data.FailureSummary != "dial tcp timeout" {
		t.Fatalf("failure summary = %q, want task error", taskPayload.Data.FailureSummary)
	}
	if taskPayload.Data.Resource == nil || len(taskPayload.Data.RelatedAudits) != 1 || len(taskPayload.Data.RelatedAlerts) != 1 || len(taskPayload.Data.Notifications) != 1 {
		t.Fatalf("task context missing linked data: %+v", taskPayload.Data)
	}
	if len(taskPayload.Data.NextActions) == 0 || taskPayload.Data.NextActions[0].Key != "test_ssh" {
		t.Fatalf("task next actions = %+v, want host test action", taskPayload.Data.NextActions)
	}

	alertContextResp := performRequest(router, http.MethodGet, "/api/alerts/events/"+alertEvent.ID+"/context", nil, token)
	if alertContextResp.Code != http.StatusOK {
		t.Fatalf("GET /api/alerts/events/:id/context status = %d, want %d; body=%s", alertContextResp.Code, http.StatusOK, alertContextResp.Body.String())
	}
	var alertPayload struct {
		Data struct {
			Task          *model.Task                `json:"task"`
			Resource      *map[string]any            `json:"resource"`
			RelatedAudits []model.AuditLog           `json:"relatedAudits"`
			Notifications []model.NotificationRecord `json:"notifications"`
			Risk          struct {
				Level string `json:"level"`
			} `json:"risk"`
		} `json:"data"`
	}
	if err := json.Unmarshal(alertContextResp.Body.Bytes(), &alertPayload); err != nil {
		t.Fatalf("decode alert context: %v", err)
	}
	if alertPayload.Data.Task == nil || alertPayload.Data.Task.ID != taskItem.ID {
		t.Fatalf("alert context task = %+v, want linked task", alertPayload.Data.Task)
	}
	if alertPayload.Data.Resource == nil || len(alertPayload.Data.RelatedAudits) != 1 || len(alertPayload.Data.Notifications) != 1 {
		t.Fatalf("alert context missing linked data: %+v", alertPayload.Data)
	}
	if alertPayload.Data.Risk.Level != "critical" {
		t.Fatalf("alert context risk = %s, want critical", alertPayload.Data.Risk.Level)
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

func TestUnmaskedExportBackupRequiresDedicatedPermissions(t *testing.T) {
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
		"username":"exporter",
		"password":"plain123456",
		"displayName":"Exporter",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	role := model.Role{Name: "Masked Exporter", Code: "masked-exporter"}
	if err := database.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	var permissions []model.Permission
	if err := database.Where("code IN ?", []string{"exports.create", "exports.download", "backups.create", "backups.download"}).Find(&permissions).Error; err != nil {
		t.Fatalf("load permissions: %v", err)
	}
	if len(permissions) != 4 {
		t.Fatalf("loaded %d export permissions, want 4", len(permissions))
	}
	for _, permission := range permissions {
		if err := database.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permission.ID}).Error; err != nil {
			t.Fatalf("assign permission %s: %v", permission.Code, err)
		}
	}
	var user model.User
	if err := database.First(&user, "username = ?", "exporter").Error; err != nil {
		t.Fatalf("load exporter user: %v", err)
	}
	if err := database.Model(&user).Association("Roles").Replace(&role); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	login := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"exporter","password":"plain123456"}`), "")
	if login.Code != http.StatusOK {
		t.Fatalf("exporter login status = %d, want %d; body=%s", login.Code, http.StatusOK, login.Body.String())
	}
	var loginPayload struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"accessToken"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode exporter login: %v", err)
	}
	limitedToken := loginPayload.Data.Tokens.AccessToken

	createService := performRequest(router, http.MethodPost, "/api/services", []byte(`{
		"name":"Sensitive Export Demo",
		"code":"sensitive-export-demo",
		"image":"registry.local/export/demo",
		"defaultTag":"1.0.0"
	}`), adminToken)
	if createService.Code != http.StatusCreated {
		t.Fatalf("POST /api/services status = %d, want %d; body=%s", createService.Code, http.StatusCreated, createService.Body.String())
	}
	serviceID := decodeID(t, createService.Body.Bytes())

	unmaskedExport := performRequest(router, http.MethodPost, "/api/exports/resources", []byte(`{
		"resourceType":"service",
		"resourceId":"`+serviceID+`",
		"masked":false
	}`), limitedToken)
	if unmaskedExport.Code != http.StatusForbidden {
		t.Fatalf("POST unmasked export limited status = %d, want %d; body=%s", unmaskedExport.Code, http.StatusForbidden, unmaskedExport.Body.String())
	}
	unmaskedBackup := performRequest(router, http.MethodPost, "/api/backups", []byte(`{"masked":false}`), limitedToken)
	if unmaskedBackup.Code != http.StatusForbidden {
		t.Fatalf("POST unmasked backup limited status = %d, want %d; body=%s", unmaskedBackup.Code, http.StatusForbidden, unmaskedBackup.Body.String())
	}

	adminUnmaskedExport := performRequest(router, http.MethodPost, "/api/exports/resources", []byte(`{
		"resourceType":"service",
		"resourceId":"`+serviceID+`",
		"masked":false
	}`), adminToken)
	if adminUnmaskedExport.Code != http.StatusCreated {
		t.Fatalf("POST unmasked export admin status = %d, want %d; body=%s", adminUnmaskedExport.Code, http.StatusCreated, adminUnmaskedExport.Body.String())
	}
	exportID := decodeID(t, adminUnmaskedExport.Body.Bytes())
	var exportJob model.ExportJob
	if err := database.First(&exportJob, "id = ?", exportID).Error; err != nil {
		t.Fatalf("load unmasked export job: %v", err)
	}
	if exportJob.Masked {
		t.Fatalf("admin unmasked export persisted as masked")
	}
	limitedDownload := performRequest(router, http.MethodGet, "/api/exports/"+exportID+"/download", nil, limitedToken)
	if limitedDownload.Code != http.StatusForbidden {
		t.Fatalf("GET unmasked export download limited status = %d, want %d; body=%s", limitedDownload.Code, http.StatusForbidden, limitedDownload.Body.String())
	}

	adminBackup := performRequest(router, http.MethodPost, "/api/backups", []byte(`{"masked":false}`), adminToken)
	if adminBackup.Code != http.StatusCreated {
		t.Fatalf("POST unmasked backup admin status = %d, want %d; body=%s", adminBackup.Code, http.StatusCreated, adminBackup.Body.String())
	}
	backupID := decodeID(t, adminBackup.Body.Bytes())
	limitedBackupDownload := performRequest(router, http.MethodGet, "/api/backups/"+backupID+"/download", nil, limitedToken)
	if limitedBackupDownload.Code != http.StatusForbidden {
		t.Fatalf("GET unmasked backup download limited status = %d, want %d; body=%s", limitedBackupDownload.Code, http.StatusForbidden, limitedBackupDownload.Body.String())
	}
}

func TestBackupManifestUsesPostgresPlaceholderWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = "postgres://aegisops:aegisops@127.0.0.1:5432/aegisops?sslmode=disable"

	database, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite database for test isolation: %v", err)
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

	backup := performRequest(router, http.MethodPost, "/api/backups", []byte(`{"masked":true}`), token)
	if backup.Code != http.StatusCreated {
		t.Fatalf("POST /api/backups status = %d, want %d; body=%s", backup.Code, http.StatusCreated, backup.Body.String())
	}
	backupID := decodeID(t, backup.Body.Bytes())

	download := performRequest(router, http.MethodGet, "/api/backups/"+backupID+"/download", nil, token)
	if download.Code != http.StatusOK {
		t.Fatalf("GET /api/backups/:id/download status = %d, want %d; body=%s", download.Code, http.StatusOK, download.Body.String())
	}

	readerAt := bytes.NewReader(download.Body.Bytes())
	zipReader, err := zip.NewReader(readerAt, int64(readerAt.Len()))
	if err != nil {
		t.Fatalf("open backup zip: %v", err)
	}

	var hasPlaceholder bool
	var manifestBody string
	for _, file := range zipReader.File {
		if file.Name == "postgresql-backup.placeholder.txt" {
			hasPlaceholder = true
		}
		if file.Name != "manifest.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open manifest.json: %v", err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read manifest.json: %v", err)
		}
		manifestBody = string(body)
	}

	if !hasPlaceholder {
		t.Fatal("backup zip did not include postgresql-backup.placeholder.txt")
	}
	if bytes.Contains([]byte(manifestBody), []byte("backup.db")) {
		t.Fatalf("manifest.json unexpectedly references backup.db: %s", manifestBody)
	}
	if !bytes.Contains([]byte(manifestBody), []byte("postgresql-backup.placeholder.txt")) {
		t.Fatalf("manifest.json did not reference postgres placeholder: %s", manifestBody)
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

	due := time.Now().UTC().Add(-time.Minute)
	if err := database.Model(&model.ScheduledJob{}).Where("id = ?", jobID).Update("next_run_at", &due).Error; err != nil {
		t.Fatalf("mark scheduled job due: %v", err)
	}
	schedulerService := schedulersvc.NewService(database)
	dispatches, err := schedulerService.DispatchDueJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDueJobs: %v", err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("dispatches len = %d, want 1", len(dispatches))
	}
	listDispatches := performRequest(router, http.MethodGet, "/api/scheduled-jobs/"+jobID+"/dispatches", nil, adminToken)
	if listDispatches.Code != http.StatusOK {
		t.Fatalf("GET /api/scheduled-jobs/:id/dispatches status = %d, want %d; body=%s", listDispatches.Code, http.StatusOK, listDispatches.Body.String())
	}
	var dispatchPayload struct {
		Data struct {
			Items []model.TaskDispatch `json:"items"`
			Total int64                `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listDispatches.Body.Bytes(), &dispatchPayload); err != nil {
		t.Fatalf("decode dispatch list: %v; body=%s", err, listDispatches.Body.String())
	}
	if dispatchPayload.Data.Total != 1 || len(dispatchPayload.Data.Items) != 1 || dispatchPayload.Data.Items[0].ID != dispatches[0].ID {
		t.Fatalf("unexpected dispatch list payload: %+v", dispatchPayload.Data)
	}
	getTask := performRequest(router, http.MethodGet, "/api/tasks/"+dispatches[0].TaskID, nil, adminToken)
	if getTask.Code != http.StatusOK {
		t.Fatalf("GET /api/tasks/:id status = %d, want %d; body=%s", getTask.Code, http.StatusOK, getTask.Body.String())
	}
	var taskPayload struct {
		Data model.Task `json:"data"`
	}
	if err := json.Unmarshal(getTask.Body.Bytes(), &taskPayload); err != nil {
		t.Fatalf("decode task detail: %v; body=%s", err, getTask.Body.String())
	}
	if len(taskPayload.Data.Dispatches) != 1 || taskPayload.Data.Dispatches[0].JobID != jobID {
		t.Fatalf("task dispatches = %+v, want scheduled dispatch for job %s", taskPayload.Data.Dispatches, jobID)
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

func TestSecretRotationRequiresDedicatedPermission(t *testing.T) {
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

	createSecret := performRequest(router, http.MethodPost, "/api/secrets", []byte(`{
		"name":"Rotate Secret",
		"type":"API_TOKEN",
		"purpose":"test",
		"value":"initial-token"
	}`), adminToken)
	if createSecret.Code != http.StatusCreated {
		t.Fatalf("POST /api/secrets status = %d, want %d; body=%s", createSecret.Code, http.StatusCreated, createSecret.Body.String())
	}
	secretID := decodeID(t, createSecret.Body.Bytes())

	createUser := performRequest(router, http.MethodPost, "/api/users", []byte(`{
		"username":"secretmgr",
		"password":"plain123456",
		"displayName":"Secret Manager",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	role := model.Role{Name: "Secret Manager", Code: "secret-manager"}
	if err := database.Create(&role).Error; err != nil {
		t.Fatalf("create secret manager role: %v", err)
	}
	var managePermission model.Permission
	if err := database.First(&managePermission, "code = ?", "secrets.manage").Error; err != nil {
		t.Fatalf("load secrets.manage permission: %v", err)
	}
	if err := database.Create(&model.RolePermission{RoleID: role.ID, PermissionID: managePermission.ID}).Error; err != nil {
		t.Fatalf("assign secrets.manage permission: %v", err)
	}
	var user model.User
	if err := database.First(&user, "username = ?", "secretmgr").Error; err != nil {
		t.Fatalf("load secret manager user: %v", err)
	}
	if err := database.Model(&user).Association("Roles").Replace(&role); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	login := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"secretmgr","password":"plain123456"}`), "")
	if login.Code != http.StatusOK {
		t.Fatalf("secretmgr login status = %d, want %d; body=%s", login.Code, http.StatusOK, login.Body.String())
	}
	var loginPayload struct {
		Data struct {
			Tokens struct {
				AccessToken string `json:"accessToken"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode secretmgr login: %v", err)
	}
	limitedToken := loginPayload.Data.Tokens.AccessToken

	rename := performRequest(router, http.MethodPatch, "/api/secrets/"+secretID, []byte(`{"name":"Rotate Secret Renamed"}`), limitedToken)
	if rename.Code != http.StatusOK {
		t.Fatalf("PATCH /api/secrets/:id rename status = %d, want %d; body=%s", rename.Code, http.StatusOK, rename.Body.String())
	}
	patchRotate := performRequest(router, http.MethodPatch, "/api/secrets/"+secretID, []byte(`{"value":"new-token"}`), limitedToken)
	if patchRotate.Code != http.StatusForbidden {
		t.Fatalf("PATCH /api/secrets/:id value limited status = %d, want %d; body=%s", patchRotate.Code, http.StatusForbidden, patchRotate.Body.String())
	}
	rotateLimited := performRequest(router, http.MethodPost, "/api/secrets/"+secretID+"/rotate", []byte(`{"value":"new-token"}`), limitedToken)
	if rotateLimited.Code != http.StatusForbidden {
		t.Fatalf("POST /api/secrets/:id/rotate limited status = %d, want %d; body=%s", rotateLimited.Code, http.StatusForbidden, rotateLimited.Body.String())
	}
	rotateAdmin := performRequest(router, http.MethodPost, "/api/secrets/"+secretID+"/rotate", []byte(`{"value":"new-token"}`), adminToken)
	if rotateAdmin.Code != http.StatusOK {
		t.Fatalf("POST /api/secrets/:id/rotate admin status = %d, want %d; body=%s", rotateAdmin.Code, http.StatusOK, rotateAdmin.Body.String())
	}
	var secretItem model.Secret
	if err := database.First(&secretItem, "id = ?", secretID).Error; err != nil {
		t.Fatalf("load rotated secret: %v", err)
	}
	if secretItem.KeyVersion != 2 || secretItem.LastRotatedAt == nil {
		t.Fatalf("secret rotation metadata = keyVersion:%d lastRotatedAt:%v, want v2 with timestamp", secretItem.KeyVersion, secretItem.LastRotatedAt)
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

func TestTaskDispatchOperationsSmoke(t *testing.T) {
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
	now := time.Now().UTC()
	taskItem := model.Task{ID: "dispatch-task-1", Type: "scheduled.noop", Title: "Dispatch Task", Status: model.TaskStatusPending}
	if err := database.Create(&taskItem).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	dispatch := model.TaskDispatch{
		ID:             "dispatch-api-1",
		TaskID:         taskItem.ID,
		Source:         model.TaskDispatchSourceManual,
		Status:         model.TaskDispatchStatusFailed,
		RetryCount:     1,
		MaxRetry:       2,
		TimeoutSeconds: 60,
		ConcurrencyKey: "dispatch:test",
		QueuedAt:       now,
	}
	if err := database.Create(&dispatch).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	list := performRequest(router, http.MethodGet, "/api/task-dispatches?status=FAILED&source=MANUAL&taskId="+taskItem.ID+"&concurrencyKey=dispatch:test", nil, adminToken)
	if list.Code != http.StatusOK {
		t.Fatalf("GET /api/task-dispatches status = %d, want %d; body=%s", list.Code, http.StatusOK, list.Body.String())
	}
	var listPayload struct {
		Data struct {
			Items []model.TaskDispatch `json:"items"`
			Total int64                `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode dispatch list: %v; body=%s", err, list.Body.String())
	}
	if listPayload.Data.Total != 1 || len(listPayload.Data.Items) != 1 || listPayload.Data.Items[0].ID != dispatch.ID {
		t.Fatalf("unexpected dispatch list: %+v", listPayload.Data)
	}
	get := performRequest(router, http.MethodGet, "/api/task-dispatches/"+dispatch.ID, nil, adminToken)
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/task-dispatches/:id status = %d, want %d; body=%s", get.Code, http.StatusOK, get.Body.String())
	}
	retry := performRequest(router, http.MethodPost, "/api/task-dispatches/"+dispatch.ID+"/retry", nil, adminToken)
	if retry.Code != http.StatusOK {
		t.Fatalf("POST /api/task-dispatches/:id/retry status = %d, want %d; body=%s", retry.Code, http.StatusOK, retry.Body.String())
	}
	var retried model.TaskDispatch
	if err := database.First(&retried, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load retried dispatch: %v", err)
	}
	if retried.Status != model.TaskDispatchStatusPending || retried.LeaseOwner != "" || retried.FinishedAt != nil {
		t.Fatalf("dispatch not retried: %+v", retried)
	}
	cancel := performRequest(router, http.MethodPost, "/api/task-dispatches/"+dispatch.ID+"/cancel", nil, adminToken)
	if cancel.Code != http.StatusOK {
		t.Fatalf("POST /api/task-dispatches/:id/cancel status = %d, want %d; body=%s", cancel.Code, http.StatusOK, cancel.Body.String())
	}
	var canceled model.TaskDispatch
	if err := database.First(&canceled, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load canceled dispatch: %v", err)
	}
	if canceled.Status != model.TaskDispatchStatusCanceled || canceled.FinishedAt == nil {
		t.Fatalf("dispatch not canceled: %+v", canceled)
	}
	var canceledTask model.Task
	if err := database.First(&canceledTask, "id = ?", taskItem.ID).Error; err != nil {
		t.Fatalf("load canceled task: %v", err)
	}
	if canceledTask.Status != model.TaskStatusCanceled {
		t.Fatalf("task status = %s, want CANCELED", canceledTask.Status)
	}
}

func TestTaskDispatchOperationsRequireDedicatedPermissions(t *testing.T) {
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
		"username":"dispatchviewer",
		"password":"plain123456",
		"displayName":"Dispatch Viewer",
		"status":"active"
	}`), adminToken)
	if createUser.Code != http.StatusCreated {
		t.Fatalf("POST /api/users status = %d, want %d; body=%s", createUser.Code, http.StatusCreated, createUser.Body.String())
	}
	plainLogin := performRequest(router, http.MethodPost, "/api/auth/login", []byte(`{"username":"dispatchviewer","password":"plain123456"}`), "")
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
		"/api/task-dispatches/dispatch-1/cancel",
		"/api/task-dispatches/dispatch-1/retry",
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
