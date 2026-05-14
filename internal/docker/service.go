package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
}

type CreateNodeRequest struct {
	Name        string               `json:"name" binding:"required"`
	Endpoint    string               `json:"endpoint" binding:"required"`
	AuthType    model.DockerAuthType `json:"authType"`
	SecretID    string               `json:"secretId"`
	Description string               `json:"description"`
	OperatorID  string               `json:"-"`
}

type UpdateNodeRequest struct {
	Name        string               `json:"name"`
	Endpoint    string               `json:"endpoint"`
	AuthType    model.DockerAuthType `json:"authType"`
	SecretID    string               `json:"secretId"`
	Description string               `json:"description"`
	OperatorID  string               `json:"-"`
}

func NewService(db *gorm.DB, secrets *secret.Service) *Service {
	return &Service{db: db, secrets: secrets}
}

func (s *Service) CreateNode(ctx context.Context, req CreateNodeRequest) (*model.DockerNode, error) {
	if req.AuthType == "" {
		req.AuthType = model.DockerAuthTypeNone
	}
	item := &model.DockerNode{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Endpoint:    req.Endpoint,
		AuthType:    req.AuthType,
		SecretID:    req.SecretID,
		Description: req.Description,
		Status:      model.DockerNodeStatusUnknown,
		CreatedBy:   req.OperatorID,
		UpdatedBy:   req.OperatorID,
	}
	return item, s.db.WithContext(ctx).Create(item).Error
}

func (s *Service) ListNodes(ctx context.Context, keyword string, limit, offset int) ([]model.DockerNode, int64, error) {
	var items []model.DockerNode
	var total int64
	query := s.db.WithContext(ctx).Model(&model.DockerNode{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR endpoint LIKE ?", like, like)
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

func (s *Service) GetNode(ctx context.Context, id string) (*model.DockerNode, error) {
	var item model.DockerNode
	if err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateNode(ctx context.Context, id string, req UpdateNodeRequest) (*model.DockerNode, error) {
	item, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Endpoint != "" {
		item.Endpoint = req.Endpoint
	}
	if req.AuthType != "" {
		item.AuthType = req.AuthType
	}
	item.SecretID = req.SecretID
	item.Description = req.Description
	item.UpdatedBy = req.OperatorID
	return item, s.db.WithContext(ctx).Save(item).Error
}

func (s *Service) DeleteNode(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.DockerNode{}, "id = ?", id).Error
}

func (s *Service) TestConnection(ctx context.Context, id string) error {
	cli, err := s.clientForNode(ctx, id)
	now := time.Now().UTC()
	status := model.DockerNodeStatusOnline
	if err == nil {
		_, err = cli.Ping(ctx)
		_ = cli.Close()
	}
	if err != nil {
		status = model.DockerNodeStatusOffline
	}
	_ = s.db.WithContext(ctx).Model(&model.DockerNode{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"last_test_at": &now,
	}).Error
	return err
}

func (s *Service) ListContainers(ctx context.Context, nodeID string, all bool) ([]dockertypes.Container, error) {
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	return cli.ContainerList(ctx, dockertypes.ContainerListOptions{All: all})
}

func (s *Service) ContainerLogs(ctx context.Context, nodeID, containerID string, tail string) (string, error) {
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return "", err
	}
	defer cli.Close()
	if tail == "" {
		tail = "200"
	}
	reader, err := cli.ContainerLogs(ctx, containerID, dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()
	var builder strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		builder.WriteString(scanner.Text())
		builder.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return "", err
	}
	return builder.String(), nil
}

func (s *Service) StartContainer(ctx context.Context, nodeID, containerID string) error {
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStart(ctx, containerID, dockertypes.ContainerStartOptions{})
}

func (s *Service) StopContainer(ctx context.Context, nodeID, containerID string) error {
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStop(ctx, containerID, containertypes.StopOptions{})
}

func (s *Service) RestartContainer(ctx context.Context, nodeID, containerID string) error {
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerRestart(ctx, containerID, containertypes.StopOptions{})
}

func (s *Service) clientForNode(ctx context.Context, id string) (*client.Client, error) {
	node, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	opts := []client.Opt{
		client.WithHost(node.Endpoint),
		client.WithAPIVersionNegotiation(),
	}
	if node.AuthType == model.DockerAuthTypeToken && node.SecretID != "" {
		token, err := s.secrets.DecryptValue(ctx, node.SecretID)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}))
	}
	if node.AuthType == model.DockerAuthTypeTLS {
		return nil, fmt.Errorf("docker tls secret is reserved but tls client setup is not enabled yet")
	}
	return client.NewClientWithOpts(opts...)
}
