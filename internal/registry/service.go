package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
)

var (
	ErrInvalidRegistryURL  = errors.New("invalid registry url")
	ErrUnsupportedAuthType = errors.New("unsupported registry auth type")
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
	client  *http.Client
}

type CreateRequest struct {
	Name        string                 `json:"name" binding:"required"`
	URL         string                 `json:"url" binding:"required"`
	AuthType    model.RegistryAuthType `json:"authType"`
	SecretID    string                 `json:"secretId"`
	Description string                 `json:"description"`
	OperatorID  string                 `json:"-"`
}

type UpdateRequest struct {
	Name        string                 `json:"name"`
	URL         string                 `json:"url"`
	AuthType    model.RegistryAuthType `json:"authType"`
	SecretID    string                 `json:"secretId"`
	Description string                 `json:"description"`
	OperatorID  string                 `json:"-"`
}

type CatalogResponse struct {
	Repositories []string `json:"repositories"`
}

type TagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type ManifestResponse struct {
	Repository  string          `json:"repository"`
	Reference   string          `json:"reference"`
	Digest      string          `json:"digest"`
	ContentType string          `json:"contentType"`
	Manifest    json.RawMessage `json:"manifest"`
}

func NewService(db *gorm.DB, secrets *secret.Service) *Service {
	return &Service{
		db:      db,
		secrets: secrets,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Registry, error) {
	if req.AuthType == "" {
		req.AuthType = model.RegistryAuthTypeNone
	}
	if err := validateAuthType(req.AuthType); err != nil {
		return nil, err
	}
	normalizedURL, err := normalizeURL(req.URL)
	if err != nil {
		return nil, err
	}
	if req.AuthType != model.RegistryAuthTypeNone && strings.TrimSpace(req.SecretID) == "" {
		return nil, fmt.Errorf("secretId is required when authType is %s", req.AuthType)
	}
	item := &model.Registry{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(req.Name),
		URL:         normalizedURL,
		AuthType:    req.AuthType,
		SecretID:    strings.TrimSpace(req.SecretID),
		Description: req.Description,
		Status:      model.RegistryStatusUnknown,
		CreatedBy:   req.OperatorID,
		UpdatedBy:   req.OperatorID,
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) List(ctx context.Context, keyword string, limit, offset int) ([]model.Registry, int64, error) {
	var items []model.Registry
	var total int64
	query := s.db.WithContext(ctx).Model(&model.Registry{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(ctx context.Context, id string) (*model.Registry, error) {
	var item model.Registry
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*model.Registry, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = strings.TrimSpace(req.Name)
	}
	if req.URL != "" {
		normalizedURL, err := normalizeURL(req.URL)
		if err != nil {
			return nil, err
		}
		item.URL = normalizedURL
		item.Status = model.RegistryStatusUnknown
		item.LastTestAt = nil
	}
	if req.AuthType != "" {
		if err := validateAuthType(req.AuthType); err != nil {
			return nil, err
		}
		item.AuthType = req.AuthType
	}
	item.SecretID = strings.TrimSpace(req.SecretID)
	item.Description = req.Description
	item.UpdatedBy = req.OperatorID
	if item.AuthType != model.RegistryAuthTypeNone && item.SecretID == "" {
		return nil, fmt.Errorf("secretId is required when authType is %s", item.AuthType)
	}
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.ServiceDefinition{}).Where("registry_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("registry is referenced by %d service definitions", count)
	}
	return s.db.WithContext(ctx).Delete(&model.Registry{}, "id = ?", id).Error
}

func (s *Service) Test(ctx context.Context, id string) error {
	registry, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.doJSON(ctx, registry, http.MethodGet, "/v2/", nil)
	now := time.Now().UTC()
	status := model.RegistryStatusOnline
	if err != nil {
		status = model.RegistryStatusOffline
	}
	_ = s.db.WithContext(ctx).Model(&model.Registry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"last_test_at": &now,
	}).Error
	return err
}

func (s *Service) Repositories(ctx context.Context, id string) (*CatalogResponse, error) {
	registry, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var result CatalogResponse
	if err := s.doJSON(ctx, registry, http.MethodGet, "/v2/_catalog", &result); err != nil {
		return nil, err
	}
	if result.Repositories == nil {
		result.Repositories = []string{}
	}
	return &result, nil
}

func (s *Service) Tags(ctx context.Context, id, repository string) (*TagsResponse, error) {
	registry, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var result TagsResponse
	requestPath := "/v2/" + cleanRepository(repository) + "/tags/list"
	if err := s.doJSON(ctx, registry, http.MethodGet, requestPath, &result); err != nil {
		return nil, err
	}
	if result.Tags == nil {
		result.Tags = []string{}
	}
	return &result, nil
}

func (s *Service) Manifest(ctx context.Context, id, repository, reference string) (*ManifestResponse, error) {
	registry, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	requestPath := "/v2/" + cleanRepository(repository) + "/manifests/" + url.PathEscape(reference)
	headers, err := s.doRaw(ctx, registry, http.MethodGet, requestPath, &raw)
	if err != nil {
		return nil, err
	}
	return &ManifestResponse{
		Repository:  repository,
		Reference:   reference,
		Digest:      headers.Get("Docker-Content-Digest"),
		ContentType: headers.Get("Content-Type"),
		Manifest:    raw,
	}, nil
}

func (s *Service) doJSON(ctx context.Context, registry *model.Registry, method, requestPath string, out any) error {
	var raw json.RawMessage
	_, err := s.doRaw(ctx, registry, method, requestPath, &raw)
	if err != nil || out == nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (s *Service) doRaw(ctx context.Context, registry *model.Registry, method, requestPath string, out *json.RawMessage) (http.Header, error) {
	endpoint, err := joinRegistryPath(registry.URL, requestPath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/json",
	}, ", "))
	if err := s.applyAuth(ctx, req, registry); err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry network error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read registry response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, registryHTTPError(resp.StatusCode, string(body))
	}
	if out != nil {
		*out = json.RawMessage(body)
	}
	return resp.Header, nil
}

func (s *Service) applyAuth(ctx context.Context, req *http.Request, registry *model.Registry) error {
	switch registry.AuthType {
	case "", model.RegistryAuthTypeNone:
		return nil
	case model.RegistryAuthTypeBasic:
		value, err := s.secrets.DecryptValue(ctx, registry.SecretID)
		if err != nil {
			return err
		}
		username, password, ok := strings.Cut(value, ":")
		if !ok {
			return errors.New("basic registry secret must use username:password format")
		}
		req.SetBasicAuth(username, password)
		return nil
	case model.RegistryAuthTypeToken:
		value, err := s.secrets.DecryptValue(ctx, registry.SecretID)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(value))
		return nil
	default:
		return ErrUnsupportedAuthType
	}
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidRegistryURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", ErrInvalidRegistryURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidRegistryURL
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateAuthType(authType model.RegistryAuthType) error {
	switch authType {
	case model.RegistryAuthTypeNone, model.RegistryAuthTypeBasic, model.RegistryAuthTypeToken:
		return nil
	default:
		return ErrUnsupportedAuthType
	}
}

func joinRegistryPath(baseURL, requestPath string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", ErrInvalidRegistryURL
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = path.Join(basePath, requestPath)
	if strings.HasSuffix(requestPath, "/") && !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.String(), nil
}

func cleanRepository(repository string) string {
	parts := strings.Split(strings.Trim(repository, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func registryHTTPError(status int, body string) error {
	message := strings.TrimSpace(body)
	if len(message) > 512 {
		message = message[:512]
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if message == "" {
			message = "registry authentication failed"
		}
		return fmt.Errorf("registry authentication failed: %s", message)
	case http.StatusNotFound:
		if message == "" {
			message = "registry resource not found"
		}
		return fmt.Errorf("registry resource not found: %s", message)
	default:
		if message == "" {
			message = http.StatusText(status)
		}
		return fmt.Errorf("registry returned status %d: %s", status, message)
	}
}
