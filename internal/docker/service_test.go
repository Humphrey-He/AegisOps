package docker

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

func TestWorkerExecutesDockerNodeTestDispatch(t *testing.T) {
	database, secretService := openDockerTestServices(t)
	service := NewService(database, secretService)
	tasks := tasksvc.NewService(database)
	node := model.DockerNode{
		ID:       "docker-worker-success",
		Name:     "Docker Worker Success",
		Endpoint: "mock://worker-success",
		AuthType: model.DockerAuthTypeNone,
		Status:   model.DockerNodeStatusUnknown,
	}
	if err := database.Create(&node).Error; err != nil {
		t.Fatalf("seed docker node: %v", err)
	}
	taskItem := model.Task{
		ID:         "task-docker-worker-success",
		Type:       "docker.node.test",
		Title:      "test docker node",
		Status:     model.TaskStatusPending,
		TargetType: "docker_node",
		TargetID:   node.ID,
	}
	if err := database.Create(&taskItem).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	dispatch := model.TaskDispatch{
		ID:             "dispatch-docker-worker-success",
		TaskID:         taskItem.ID,
		Source:         model.TaskDispatchSourceScheduled,
		Status:         model.TaskDispatchStatusPending,
		TimeoutSeconds: 60,
		ConcurrencyKey: "docker:" + node.ID + ":test",
		QueuedAt:       time.Now().UTC(),
	}
	if err := database.Create(&dispatch).Error; err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	worker := tasksvc.NewWorker(tasks)
	processed, err := worker.RunOnce(context.Background(), tasksvc.WorkerOptions{
		Owner:    "docker-test-worker",
		Executor: tasksvc.NewDispatchExecutor(tasksvc.DispatchExecutorOptions{Docker: service}),
	})
	if err != nil {
		t.Fatalf("worker RunOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	var updatedNode model.DockerNode
	if err := database.First(&updatedNode, "id = ?", node.ID).Error; err != nil {
		t.Fatalf("load docker node: %v", err)
	}
	if updatedNode.Status != model.DockerNodeStatusOnline || updatedNode.LastTestAt == nil {
		t.Fatalf("docker node status = %s lastTestAt=%v, want ONLINE with timestamp", updatedNode.Status, updatedNode.LastTestAt)
	}
	var updatedTask model.Task
	if err := database.First(&updatedTask, "id = ?", taskItem.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != model.TaskStatusSuccess || updatedTask.Result == "" {
		t.Fatalf("task after docker dispatch = %+v", updatedTask)
	}
	var updatedDispatch model.TaskDispatch
	if err := database.First(&updatedDispatch, "id = ?", dispatch.ID).Error; err != nil {
		t.Fatalf("load dispatch: %v", err)
	}
	if updatedDispatch.Status != model.TaskDispatchStatusSuccess || updatedDispatch.FinishedAt == nil {
		t.Fatalf("dispatch after docker test = %+v", updatedDispatch)
	}
}

func openDockerTestServices(t *testing.T) (*gorm.DB, *secretsvc.Service) {
	t.Helper()

	database, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "aegisops.db"),
	})
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
	secretService, err := secretsvc.NewService(database, "test-master-key")
	if err != nil {
		t.Fatalf("new secret service: %v", err)
	}
	return database, secretService
}

func TestDockerTLSConfigFromSecretJSON(t *testing.T) {
	secretValue := newTestDockerTLSSecret(t, false)

	tlsConfig, err := dockerTLSConfigFromSecret(secretValue)
	if err != nil {
		t.Fatalf("dockerTLSConfigFromSecret() error = %v", err)
	}

	if tlsConfig.ServerName != "docker.example.com" {
		t.Fatalf("ServerName = %q", tlsConfig.ServerName)
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = true")
	}
	if tlsConfig.RootCAs == nil {
		t.Fatal("RootCAs is nil")
	}
	if len(tlsConfig.Certificates) != 1 {
		t.Fatalf("Certificates length = %d", len(tlsConfig.Certificates))
	}
}

func TestDockerTLSConfigFromSecretMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload dockerTLSSecret
		want    string
	}{
		{
			name:    "cert",
			payload: dockerTLSSecret{Key: "key", CACert: "ca"},
			want:    "missing cert",
		},
		{
			name:    "key",
			payload: dockerTLSSecret{Cert: "cert", CACert: "ca"},
			want:    "missing key",
		},
		{
			name:    "ca",
			payload: dockerTLSSecret{Cert: "cert", Key: "key"},
			want:    "missing caCert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			_, err = dockerTLSConfigFromSecret(string(raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("dockerTLSConfigFromSecret() error = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func TestDockerTLSConfigFromSecretInvalidPEMDoesNotLeakKey(t *testing.T) {
	privateKeyMarker := "PRIVATE-SECRET-DO-NOT-LEAK"
	raw, err := json.Marshal(dockerTLSSecret{
		Cert:               "not a certificate",
		Key:                privateKeyMarker,
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	_, err = dockerTLSConfigFromSecret(string(raw))
	if err == nil {
		t.Fatal("dockerTLSConfigFromSecret() error = nil")
	}
	if !strings.Contains(err.Error(), "parse docker tls client certificate") {
		t.Fatalf("error = %v, want client certificate context", err)
	}
	if strings.Contains(err.Error(), privateKeyMarker) {
		t.Fatalf("error leaked private key content: %v", err)
	}
}

func TestClientOptsForNodeBuildsTLSTransport(t *testing.T) {
	database, err := db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/aegisops.db"})
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.AutoMigrate(&model.Secret{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	secretService, err := secretsvc.NewService(database, "test-master-key")
	if err != nil {
		t.Fatalf("secret.NewService() error = %v", err)
	}
	secretItem, err := secretService.Create(context.Background(), secretsvc.CreateRequest{
		Name:  "docker tls",
		Type:  model.SecretTypeDockerTLS,
		Value: newTestDockerTLSSecret(t, true),
	})
	if err != nil {
		t.Fatalf("secret.Create() error = %v", err)
	}

	service := NewService(database, secretService)
	opts, err := service.clientOptsForNode(context.Background(), &model.DockerNode{
		Endpoint: "tcp://127.0.0.1:2376",
		AuthType: model.DockerAuthTypeTLS,
		SecretID: secretItem.ID,
	})
	if err != nil {
		t.Fatalf("clientOptsForNode() error = %v", err)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		t.Fatalf("NewClientWithOpts() error = %v", err)
	}
	defer cli.Close()

	if cli.DaemonHost() != "tcp://127.0.0.1:2376" {
		t.Fatalf("DaemonHost() = %q", cli.DaemonHost())
	}
	transport, ok := cli.HTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T", cli.HTTPClient().Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false")
	}
}

func newTestDockerTLSSecret(t *testing.T, insecureSkipVerify bool) string {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey(ca) error = %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(ca) error = %v", err)
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey(client) error = %v", err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "docker client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CreateCertificate(client) error = %v", err)
	}

	payload := dockerTLSSecret{
		CACert:             string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
		Cert:               string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})),
		Key:                string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})),
		ServerName:         "docker.example.com",
		InsecureSkipVerify: insecureSkipVerify,
	}
	if _, err := tls.X509KeyPair([]byte(payload.Cert), []byte(payload.Key)); err != nil {
		t.Fatalf("generated key pair is invalid: %v", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(raw)
}
