package demo

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
)

func Seed(ctx context.Context, db *gorm.DB, env string) error {
	if !shouldSeed(env) {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertEnvironment(tx); err != nil {
			return err
		}
		registry, err := upsertRegistry(tx)
		if err != nil {
			return err
		}
		node, err := upsertDockerNode(tx)
		if err != nil {
			return err
		}
		if err := upsertDemoContainer(tx, node.ID); err != nil {
			return err
		}
		return upsertServices(tx, registry.ID, node.ID)
	})
}

func shouldSeed(env string) bool {
	env = strings.ToLower(strings.TrimSpace(env))
	return env == "" || env == "dev" || env == "development" || env == "test"
}

func upsertEnvironment(tx *gorm.DB) error {
	item := model.Environment{
		ID:          stableID("environment-dev"),
		Name:        "Development",
		Code:        "dev",
		Description: "Default demo environment",
		Status:      model.EnvironmentStatusActive,
		SortOrder:   10,
		CreatedBy:   "system",
		UpdatedBy:   "system",
	}
	return tx.Where("id = ?", item.ID).Assign(item).FirstOrCreate(&item).Error
}

func upsertRegistry(tx *gorm.DB) (*model.Registry, error) {
	item := model.Registry{
		ID:          stableID("demo-registry"),
		Name:        "Demo Registry",
		URL:         "https://registry-1.docker.io",
		AuthType:    model.RegistryAuthTypeNone,
		Environment: "dev",
		Description: "Demo registry seeded for live API walkthrough",
		Status:      model.RegistryStatusOnline,
		CreatedBy:   "system",
		UpdatedBy:   "system",
	}
	err := tx.Where("id = ?", item.ID).Assign(item).FirstOrCreate(&item).Error
	return &item, err
}

func upsertDockerNode(tx *gorm.DB) (*model.DockerNode, error) {
	now := time.Now().UTC()
	item := model.DockerNode{
		ID:          stableID("demo-docker-node"),
		Name:        "mock-docker-live-01",
		Endpoint:    "mock://docker-live-01",
		AuthType:    model.DockerAuthTypeNone,
		Environment: "dev",
		Description: "Live API demo node. Backend executes container actions in mock mode.",
		Status:      model.DockerNodeStatusOnline,
		LastTestAt:  &now,
		CreatedBy:   "system",
		UpdatedBy:   "system",
	}
	err := tx.Where("id = ?", item.ID).Assign(item).FirstOrCreate(&item).Error
	return &item, err
}

func upsertDemoContainer(tx *gorm.DB, nodeID string) error {
	now := time.Now().UTC()
	item := model.MockDockerContainer{
		ID:           stableID("demo-container-aegisops-api"),
		NodeID:       nodeID,
		ServiceID:    stableID("demo-service-aegisops-api"),
		Name:         "aegisops-api-v0.1.0",
		Image:        "nginx:1.25-alpine",
		Status:       model.MockDockerContainerRunning,
		Ports:        `[{"containerPort":80,"hostPort":18080,"protocol":"TCP"}]`,
		RestartCount: 1,
		Logs: strings.Join([]string{
			"[" + now.Add(-20*time.Minute).Format(time.RFC3339) + "] INFO demo container booted",
			"[" + now.Add(-18*time.Minute).Format(time.RFC3339) + "] INFO health check passed",
			"[" + now.Add(-5*time.Minute).Format(time.RFC3339) + "] INFO ready for container action walkthrough",
		}, "\n"),
	}
	return tx.Where("id = ?", item.ID).Assign(item).FirstOrCreate(&item).Error
}

func upsertServices(tx *gorm.DB, registryID, nodeID string) error {
	services := []model.ServiceDefinition{
		{
			ID:             stableID("demo-service-aegisops-api"),
			Name:           "AegisOps API Demo",
			Code:           "aegisops-api",
			Environment:    "dev",
			Group:          "demo",
			Tags:           `["demo","live-api"]`,
			Description:    "Demo service for live API release flow",
			RegistryID:     registryID,
			Image:          "nginx",
			DefaultTag:     "1.25-alpine",
			Ports:          mustJSON([]map[string]any{{"name": "http", "containerPort": 80, "hostPort": 18080, "protocol": "TCP"}}),
			Envs:           mustJSON([]map[string]string{{"key": "APP_ENV", "value": "demo"}}),
			Mounts:         "[]",
			ResourceLimits: mustJSON(map[string]string{"cpu": "0.5", "memory": "128m"}),
			TargetType:     "DOCKER_NODE",
			TargetID:       nodeID,
			Status:         model.ServiceStatusActive,
			CurrentVersion: "v0.1.0",
			CreatedBy:      "system",
			UpdatedBy:      "system",
		},
		{
			ID:             stableID("demo-service-console"),
			Name:           "Console Preview Demo",
			Code:           "console-preview",
			Environment:    "dev",
			Group:          "demo",
			Tags:           `["demo","preview"]`,
			Description:    "Demo service ready for release/upgrade testing",
			RegistryID:     registryID,
			Image:          "nginx",
			DefaultTag:     "stable-alpine",
			Ports:          mustJSON([]map[string]any{{"name": "web", "containerPort": 80, "hostPort": 18081, "protocol": "TCP"}}),
			Envs:           "[]",
			Mounts:         "[]",
			ResourceLimits: mustJSON(map[string]string{"cpu": "0.25", "memory": "96m"}),
			TargetType:     "DOCKER_NODE",
			TargetID:       nodeID,
			Status:         model.ServiceStatusDraft,
			CreatedBy:      "system",
			UpdatedBy:      "system",
		},
	}
	for _, service := range services {
		if err := tx.Where("id = ?", service.ID).Assign(service).FirstOrCreate(&service).Error; err != nil {
			return err
		}
	}
	return upsertInitialVersionAndInstance(tx, services[0], nodeID)
}

func upsertInitialVersionAndInstance(tx *gorm.DB, service model.ServiceDefinition, nodeID string) error {
	version := model.ServiceVersion{
		ID:        stableID("demo-version-aegisops-api-v010"),
		ServiceID: service.ID,
		Version:   "v0.1.0",
		Image:     service.Image,
		ImageTag:  service.DefaultTag,
		Config:    service.Ports,
		CreatedBy: "system",
	}
	if err := tx.Where("id = ?", version.ID).Assign(version).FirstOrCreate(&version).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	instance := model.ServiceInstance{
		ID:           stableID("demo-instance-aegisops-api"),
		ServiceID:    service.ID,
		VersionID:    version.ID,
		Version:      version.Version,
		Image:        service.Image + ":" + service.DefaultTag,
		ImageTag:     service.DefaultTag,
		DockerNodeID: nodeID,
		Environment:  service.Environment,
		ContainerID:  stableID("demo-container-aegisops-api"),
		Name:         service.Code,
		Status:       model.ServiceInstanceStatusRunning,
		StartedAt:    &now,
	}
	return tx.Where("id = ?", instance.ID).Assign(instance).FirstOrCreate(&instance).Error
}

func stableID(name string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("aegisops:"+name)).String()
}

func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(bytes)
}
