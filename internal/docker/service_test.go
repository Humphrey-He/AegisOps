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
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"

	"github.com/Humphrey-He/AegisOps/internal/config"
	"github.com/Humphrey-He/AegisOps/internal/db"
	"github.com/Humphrey-He/AegisOps/internal/model"
	secretsvc "github.com/Humphrey-He/AegisOps/internal/secret"
)

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
