package docker

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
	tasksvc "github.com/Humphrey-He/AegisOps/internal/task"
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
	tasks   *tasksvc.Service
	taskMu  sync.Mutex
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

type DeployRequest struct {
	NodeID        string
	ServiceID     string
	ServiceCode   string
	ContainerName string
	Image         string
	ImageTag      string
	PortsJSON     string
	EnvsJSON      string
	MountsJSON    string
	ResourcesJSON string
	RegistryAuth  string
}

type DeployResult struct {
	ContainerID string `json:"containerId"`
	Image       string `json:"image"`
}

type portMapping struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIp"`
}

type envMapping struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type mountMapping struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type resourceLimits struct {
	MemoryBytes int64  `json:"memoryBytes"`
	CPUQuota    int64  `json:"cpuQuota"`
	CPUPeriod   int64  `json:"cpuPeriod"`
	Memory      string `json:"memory"`
	CPU         string `json:"cpu"`
}

type dockerTLSSecret struct {
	CACert             string `json:"caCert"`
	Cert               string `json:"cert"`
	Key                string `json:"key"`
	ServerName         string `json:"serverName"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
}

func NewService(db *gorm.DB, secrets *secret.Service) *Service {
	return &Service{db: db, secrets: secrets}
}

func (s *Service) SetTaskService(tasks *tasksvc.Service) {
	s.tasks = tasks
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
	node, err := s.GetNode(ctx, id)
	if err != nil {
		return err
	}
	if isMockEndpoint(node.Endpoint) {
		now := time.Now().UTC()
		return s.db.WithContext(ctx).Model(&model.DockerNode{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       model.DockerNodeStatusOnline,
			"last_test_at": &now,
		}).Error
	}
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

func (s *Service) TestConnectionTask(ctx context.Context, id, operatorID string) (string, error) {
	return s.runTask(ctx, tasksvc.CreateRequest{
		Type:       "docker.node.test",
		Title:      "test docker node " + id,
		TargetType: "docker_node",
		TargetID:   id,
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "connect docker daemon", SortOrder: 1},
			{Name: "record node status", SortOrder: 2},
		},
	}, func(ctx context.Context) error {
		return s.TestConnection(ctx, id)
	})
}

func (s *Service) ListContainers(ctx context.Context, nodeID string, all bool) ([]dockertypes.Container, error) {
	node, err := s.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if isMockEndpoint(node.Endpoint) {
		return s.listMockContainers(ctx, nodeID, all)
	}
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	return cli.ContainerList(ctx, dockertypes.ContainerListOptions{All: all})
}

func (s *Service) ContainerLogs(ctx context.Context, nodeID, containerID string, tail string) (string, error) {
	node, err := s.GetNode(ctx, nodeID)
	if err != nil {
		return "", err
	}
	if isMockEndpoint(node.Endpoint) {
		var item model.MockDockerContainer
		if err := s.db.WithContext(ctx).First(&item, "id = ? AND node_id = ?", containerID, nodeID).Error; err != nil {
			return "", err
		}
		return item.Logs, nil
	}
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
	if ok, err := s.updateMockContainerStatus(ctx, nodeID, containerID, model.MockDockerContainerRunning); ok || err != nil {
		return err
	}
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStart(ctx, containerID, dockertypes.ContainerStartOptions{})
}

func (s *Service) StartContainerTask(ctx context.Context, nodeID, containerID, operatorID string) (string, error) {
	return s.containerActionTask(ctx, "container.start", "start container "+containerID, nodeID, containerID, operatorID, s.StartContainer)
}

func (s *Service) StopContainer(ctx context.Context, nodeID, containerID string) error {
	if ok, err := s.updateMockContainerStatus(ctx, nodeID, containerID, model.MockDockerContainerExited); ok || err != nil {
		return err
	}
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStop(ctx, containerID, containertypes.StopOptions{})
}

func (s *Service) StopContainerTask(ctx context.Context, nodeID, containerID, operatorID string) (string, error) {
	return s.containerActionTask(ctx, "container.stop", "stop container "+containerID, nodeID, containerID, operatorID, s.StopContainer)
}

func (s *Service) RestartContainer(ctx context.Context, nodeID, containerID string) error {
	if ok, err := s.restartMockContainer(ctx, nodeID, containerID); ok || err != nil {
		return err
	}
	cli, err := s.clientForNode(ctx, nodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerRestart(ctx, containerID, containertypes.StopOptions{})
}

func (s *Service) RestartContainerTask(ctx context.Context, nodeID, containerID, operatorID string) (string, error) {
	return s.containerActionTask(ctx, "container.restart", "restart container "+containerID, nodeID, containerID, operatorID, s.RestartContainer)
}

func (s *Service) containerActionTask(ctx context.Context, taskType, title, nodeID, containerID, operatorID string, fn func(context.Context, string, string) error) (string, error) {
	return s.runTask(ctx, tasksvc.CreateRequest{
		Type:       taskType,
		Title:      title,
		TargetType: "container",
		TargetID:   containerID,
		CreatedBy:  operatorID,
		Steps: []tasksvc.CreateStepRequest{
			{Name: "connect docker daemon", SortOrder: 1},
			{Name: "execute container action", SortOrder: 2},
		},
	}, func(ctx context.Context) error {
		return fn(ctx, nodeID, containerID)
	})
}

func (s *Service) runTask(ctx context.Context, req tasksvc.CreateRequest, fn func(context.Context) error) (string, error) {
	if s.tasks == nil {
		return "", fn(ctx)
	}
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	task, err := s.tasks.CreateRunning(ctx, req)
	if err != nil {
		return "", err
	}
	err = fn(ctx)
	if err != nil {
		_, _ = s.tasks.AddLog(ctx, task.ID, "", model.TaskLogLevelError, err.Error())
		_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusFailed, "", err.Error())
		return task.ID, err
	}
	_ = s.tasks.Finish(ctx, task.ID, model.TaskStatusSuccess, "ok", "")
	return task.ID, nil
}

func (s *Service) DeployContainer(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	if strings.TrimSpace(req.NodeID) == "" {
		return nil, fmt.Errorf("target docker node is required")
	}
	if err := s.ValidateDeploy(ctx, req); err != nil {
		return nil, err
	}
	node, err := s.GetNode(ctx, req.NodeID)
	if err != nil {
		return nil, err
	}
	if isMockEndpoint(node.Endpoint) {
		return s.deployMockContainer(ctx, req)
	}
	cli, err := s.clientForNode(ctx, req.NodeID)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	imageRef := req.Image
	if req.ImageTag != "" && !strings.Contains(lastImageSegment(req.Image), ":") {
		imageRef = req.Image + ":" + req.ImageTag
	}
	reader, err := cli.ImagePull(ctx, imageRef, dockertypes.ImagePullOptions{RegistryAuth: req.RegistryAuth})
	if err != nil {
		return nil, fmt.Errorf("pull image %s: %w", imageRef, err)
	}
	_, _ = io.Copy(io.Discard, reader)
	_ = reader.Close()

	exposedPorts, portBindings, err := parsePorts(req.PortsJSON)
	if err != nil {
		return nil, err
	}
	mounts, err := parseMounts(req.MountsJSON)
	if err != nil {
		return nil, err
	}
	resources, err := parseResources(req.ResourcesJSON)
	if err != nil {
		return nil, err
	}
	envs, err := parseEnvs(req.EnvsJSON)
	if err != nil {
		return nil, err
	}
	name := req.ContainerName
	if name == "" {
		name = req.ServiceCode
	}
	if err := s.removeExistingServiceContainer(ctx, cli, name, req.ServiceID); err != nil {
		return nil, err
	}
	created, err := cli.ContainerCreate(ctx, &containertypes.Config{
		Image:        imageRef,
		Env:          envs,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			"aegisops.service.id":   req.ServiceID,
			"aegisops.service.code": req.ServiceCode,
		},
	}, &containertypes.HostConfig{
		PortBindings: portBindings,
		Mounts:       mounts,
		Resources:    resources,
		RestartPolicy: containertypes.RestartPolicy{
			Name: "unless-stopped",
		},
	}, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	if err := cli.ContainerStart(ctx, created.ID, dockertypes.ContainerStartOptions{}); err != nil {
		_ = cli.ContainerRemove(context.Background(), created.ID, dockertypes.ContainerRemoveOptions{Force: true})
		return nil, fmt.Errorf("start container: %w", err)
	}
	return &DeployResult{ContainerID: created.ID, Image: imageRef}, nil
}

func (s *Service) ValidateDeploy(ctx context.Context, req DeployRequest) error {
	if strings.TrimSpace(req.NodeID) == "" {
		return fmt.Errorf("target docker node is required")
	}
	if strings.TrimSpace(req.Image) == "" {
		return fmt.Errorf("service image is required")
	}
	node, err := s.GetNode(ctx, req.NodeID)
	if err != nil {
		return err
	}
	if isMockEndpoint(node.Endpoint) {
		if _, _, err := parsePorts(req.PortsJSON); err != nil {
			return err
		}
		if _, err := parseEnvs(req.EnvsJSON); err != nil {
			return err
		}
		if _, err := parseMounts(req.MountsJSON); err != nil {
			return err
		}
		if _, err := parseResources(req.ResourcesJSON); err != nil {
			return err
		}
		return nil
	}
	cli, err := s.clientForNode(ctx, req.NodeID)
	if err != nil {
		return err
	}
	defer cli.Close()
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("docker node is not reachable: %w", err)
	}
	if _, _, err := parsePorts(req.PortsJSON); err != nil {
		return err
	}
	if _, err := parseEnvs(req.EnvsJSON); err != nil {
		return err
	}
	if _, err := parseMounts(req.MountsJSON); err != nil {
		return err
	}
	if _, err := parseResources(req.ResourcesJSON); err != nil {
		return err
	}
	name := firstNonEmpty(req.ContainerName, req.ServiceCode)
	if name == "" {
		return fmt.Errorf("container name is required")
	}
	inspect, err := cli.ContainerInspect(ctx, name)
	if err == nil {
		if inspect.Config == nil || inspect.Config.Labels["aegisops.service.id"] != req.ServiceID {
			return fmt.Errorf("container name %s is already used by another workload", name)
		}
		return nil
	}
	if errdefs.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("inspect container %s: %w", name, err)
}

func (s *Service) removeExistingServiceContainer(ctx context.Context, cli *client.Client, name, serviceID string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("inspect existing container %s: %w", name, err)
	}
	if inspect.Config == nil || inspect.Config.Labels["aegisops.service.id"] != serviceID {
		return fmt.Errorf("container name %s is already used by another workload", name)
	}
	stopTimeout := 10
	_ = cli.ContainerStop(ctx, inspect.ID, containertypes.StopOptions{Timeout: &stopTimeout})
	if err := cli.ContainerRemove(ctx, inspect.ID, dockertypes.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove existing container %s: %w", name, err)
	}
	return nil
}

func (s *Service) clientForNode(ctx context.Context, id string) (*client.Client, error) {
	node, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	opts, err := s.clientOptsForNode(ctx, node)
	if err != nil {
		return nil, err
	}
	return client.NewClientWithOpts(opts...)
}

func (s *Service) clientOptsForNode(ctx context.Context, node *model.DockerNode) ([]client.Opt, error) {
	opts := []client.Opt{}
	if node.AuthType == model.DockerAuthTypeToken && node.SecretID != "" {
		token, err := s.secrets.DecryptValue(ctx, node.SecretID)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}))
	}
	if node.AuthType == model.DockerAuthTypeTLS {
		if strings.TrimSpace(node.SecretID) == "" {
			return nil, fmt.Errorf("docker tls secret id is required")
		}
		value, err := s.secrets.DecryptValue(ctx, node.SecretID)
		if err != nil {
			return nil, err
		}
		tlsConfig, err := dockerTLSConfigFromSecret(value)
		if err != nil {
			return nil, err
		}
		opts = append(opts, dockerWithTLSConfig(tlsConfig))
	}
	opts = append(opts,
		client.WithHost(node.Endpoint),
		client.WithAPIVersionNegotiation(),
	)
	return opts, nil
}

func dockerWithTLSConfig(tlsConfig *tls.Config) client.Opt {
	return client.WithHTTPClient(&http.Client{
		Transport:     &http.Transport{TLSClientConfig: tlsConfig},
		CheckRedirect: client.CheckRedirect,
	})
}

func dockerTLSConfigFromSecret(value string) (*tls.Config, error) {
	var payload dockerTLSSecret
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &payload); err != nil {
		return nil, fmt.Errorf("parse docker tls secret JSON: %w", err)
	}

	certPEM := strings.TrimSpace(payload.Cert)
	keyPEM := strings.TrimSpace(payload.Key)
	caPEM := strings.TrimSpace(payload.CACert)
	if certPEM == "" {
		return nil, fmt.Errorf("docker tls secret missing cert")
	}
	if keyPEM == "" {
		return nil, fmt.Errorf("docker tls secret missing key")
	}
	if caPEM == "" && !payload.InsecureSkipVerify {
		return nil, fmt.Errorf("docker tls secret missing caCert")
	}

	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse docker tls client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ServerName:         strings.TrimSpace(payload.ServerName),
		InsecureSkipVerify: payload.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
	if caPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caPEM)) {
			return nil, fmt.Errorf("parse docker tls caCert: no valid PEM certificate found")
		}
		tlsConfig.RootCAs = pool
	}
	return tlsConfig, nil
}

func lastImageSegment(image string) string {
	parts := strings.Split(image, "/")
	return parts[len(parts)-1]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parsePorts(raw string) (nat.PortSet, nat.PortMap, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	if strings.TrimSpace(raw) == "" {
		return exposed, bindings, nil
	}
	var ports []portMapping
	if err := json.Unmarshal([]byte(raw), &ports); err != nil {
		return nil, nil, fmt.Errorf("parse ports: %w", err)
	}
	for _, item := range ports {
		if item.ContainerPort <= 0 {
			return nil, nil, fmt.Errorf("containerPort is required")
		}
		protocol := strings.ToLower(item.Protocol)
		if protocol == "" {
			protocol = "tcp"
		}
		port, err := nat.NewPort(protocol, strconv.Itoa(item.ContainerPort))
		if err != nil {
			return nil, nil, err
		}
		exposed[port] = struct{}{}
		if item.HostPort > 0 {
			bindings[port] = []nat.PortBinding{{HostIP: item.HostIP, HostPort: strconv.Itoa(item.HostPort)}}
		}
	}
	return exposed, bindings, nil
}

func parseEnvs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var envs []envMapping
	if err := json.Unmarshal([]byte(raw), &envs); err != nil {
		return nil, fmt.Errorf("parse envs: %w", err)
	}
	result := make([]string, 0, len(envs))
	for _, item := range envs {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.Key)
		}
		if name == "" {
			return nil, fmt.Errorf("env name is required")
		}
		result = append(result, name+"="+item.Value)
	}
	return result, nil
}

func parseMounts(raw string) ([]mounttypes.Mount, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var mounts []mountMapping
	if err := json.Unmarshal([]byte(raw), &mounts); err != nil {
		return nil, fmt.Errorf("parse mounts: %w", err)
	}
	result := make([]mounttypes.Mount, 0, len(mounts))
	for _, item := range mounts {
		if item.Target == "" {
			return nil, fmt.Errorf("mount target is required")
		}
		mountType := mounttypes.TypeBind
		if item.Type != "" {
			mountType = mounttypes.Type(item.Type)
		}
		result = append(result, mounttypes.Mount{
			Type:     mountType,
			Source:   item.Source,
			Target:   item.Target,
			ReadOnly: item.ReadOnly,
		})
	}
	return result, nil
}

func parseResources(raw string) (containertypes.Resources, error) {
	if strings.TrimSpace(raw) == "" {
		return containertypes.Resources{}, nil
	}
	var limits resourceLimits
	if err := json.Unmarshal([]byte(raw), &limits); err != nil {
		return containertypes.Resources{}, fmt.Errorf("parse resourceLimits: %w", err)
	}
	memoryBytes, err := parseMemoryBytes(limits.Memory)
	if err != nil {
		return containertypes.Resources{}, err
	}
	if limits.MemoryBytes == 0 {
		limits.MemoryBytes = memoryBytes
	}
	if limits.CPUQuota == 0 && strings.TrimSpace(limits.CPU) != "" {
		cpu, err := strconv.ParseFloat(strings.TrimSpace(limits.CPU), 64)
		if err != nil {
			return containertypes.Resources{}, fmt.Errorf("parse cpu: %w", err)
		}
		limits.CPUPeriod = 100000
		limits.CPUQuota = int64(cpu * float64(limits.CPUPeriod))
	}
	return containertypes.Resources{
		Memory:    limits.MemoryBytes,
		CPUQuota:  limits.CPUQuota,
		CPUPeriod: limits.CPUPeriod,
	}, nil
}

func parseMemoryBytes(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	multiplier := int64(1)
	lower := strings.ToLower(value)
	for _, unit := range []struct {
		suffix     string
		multiplier int64
	}{
		{"gib", 1024 * 1024 * 1024},
		{"gb", 1000 * 1000 * 1000},
		{"gi", 1024 * 1024 * 1024},
		{"g", 1000 * 1000 * 1000},
		{"mib", 1024 * 1024},
		{"mb", 1000 * 1000},
		{"mi", 1024 * 1024},
		{"m", 1000 * 1000},
		{"kib", 1024},
		{"kb", 1000},
		{"ki", 1024},
		{"k", 1000},
	} {
		if strings.HasSuffix(lower, unit.suffix) {
			multiplier = unit.multiplier
			value = strings.TrimSpace(value[:len(value)-len(unit.suffix)])
			break
		}
	}
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse memory: %w", err)
	}
	return int64(amount * float64(multiplier)), nil
}

func isMockEndpoint(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(endpoint)), "mock://")
}

func (s *Service) listMockContainers(ctx context.Context, nodeID string, all bool) ([]dockertypes.Container, error) {
	var items []model.MockDockerContainer
	query := s.db.WithContext(ctx).Where("node_id = ?", nodeID)
	if !all {
		query = query.Where("status = ?", model.MockDockerContainerRunning)
	}
	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	containers := make([]dockertypes.Container, 0, len(items))
	for _, item := range items {
		containers = append(containers, mockContainerToDocker(item))
	}
	return containers, nil
}

func mockContainerToDocker(item model.MockDockerContainer) dockertypes.Container {
	names := []string{}
	if item.Name != "" {
		names = []string{"/" + item.Name}
	}
	return dockertypes.Container{
		ID:      item.ID,
		Names:   names,
		Image:   item.Image,
		ImageID: "mock:" + item.ID,
		Command: "mock workload",
		Created: item.CreatedAt.Unix(),
		State:   string(item.Status),
		Status:  mockContainerStatusText(item),
		Labels: map[string]string{
			"aegisops.mock":       "true",
			"aegisops.service.id": item.ServiceID,
		},
	}
}

func mockContainerStatusText(item model.MockDockerContainer) string {
	switch item.Status {
	case model.MockDockerContainerRunning:
		return "Up " + time.Since(item.UpdatedAt).Round(time.Second).String()
	case model.MockDockerContainerPaused:
		return "Paused"
	default:
		return "Exited"
	}
}

func (s *Service) updateMockContainerStatus(ctx context.Context, nodeID, containerID string, status model.MockDockerContainerStatus) (bool, error) {
	node, err := s.GetNode(ctx, nodeID)
	if err != nil {
		return false, err
	}
	if !isMockEndpoint(node.Endpoint) {
		return false, nil
	}
	var item model.MockDockerContainer
	if err := s.db.WithContext(ctx).First(&item, "id = ? AND node_id = ?", containerID, nodeID).Error; err != nil {
		return true, err
	}
	item.Status = status
	item.Logs = appendMockLog(item.Logs, fmt.Sprintf("container %s set to %s", item.Name, status))
	return true, s.db.WithContext(ctx).Save(&item).Error
}

func (s *Service) restartMockContainer(ctx context.Context, nodeID, containerID string) (bool, error) {
	node, err := s.GetNode(ctx, nodeID)
	if err != nil {
		return false, err
	}
	if !isMockEndpoint(node.Endpoint) {
		return false, nil
	}
	var item model.MockDockerContainer
	if err := s.db.WithContext(ctx).First(&item, "id = ? AND node_id = ?", containerID, nodeID).Error; err != nil {
		return true, err
	}
	item.Status = model.MockDockerContainerRunning
	item.RestartCount++
	item.Logs = appendMockLog(item.Logs, fmt.Sprintf("container %s restarted", item.Name))
	return true, s.db.WithContext(ctx).Save(&item).Error
}

func (s *Service) deployMockContainer(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	name := firstNonEmpty(req.ContainerName, req.ServiceCode)
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("container name is required")
	}
	image := req.Image
	if req.ImageTag != "" && !strings.Contains(lastImageSegment(req.Image), ":") {
		image = req.Image + ":" + req.ImageTag
	}
	containerID := "mock-" + uuid.NewString()
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ? AND name = ?", req.NodeID, name).Delete(&model.MockDockerContainer{}).Error; err != nil {
			return err
		}
		item := model.MockDockerContainer{
			ID:        containerID,
			NodeID:    req.NodeID,
			ServiceID: req.ServiceID,
			Name:      name,
			Image:     image,
			Status:    model.MockDockerContainerRunning,
			Ports:     req.PortsJSON,
			Logs: strings.Join([]string{
				fmt.Sprintf("[%s] INFO pulling image %s", now.Format(time.RFC3339), image),
				fmt.Sprintf("[%s] INFO created mock container %s", now.Format(time.RFC3339), name),
				fmt.Sprintf("[%s] INFO container is running on mock docker node", now.Format(time.RFC3339)),
			}, "\n"),
		}
		return tx.Create(&item).Error
	})
	if err != nil {
		return nil, err
	}
	return &DeployResult{ContainerID: containerID, Image: image}, nil
}

func appendMockLog(logs, message string) string {
	line := fmt.Sprintf("[%s] INFO %s", time.Now().UTC().Format(time.RFC3339), message)
	if strings.TrimSpace(logs) == "" {
		return line
	}
	return logs + "\n" + line
}
