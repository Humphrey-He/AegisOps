package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Humphrey-He/AegisOps/internal/model"
	"github.com/Humphrey-He/AegisOps/internal/secret"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	secrets *secret.Service
	dialer  SSHDialer
	timeout time.Duration
}

type SSHDialer interface {
	Dial(ctx context.Context, network, address string, config *ssh.ClientConfig) (SSHClient, error)
}

type SSHClient interface {
	NewSession() (SSHSession, error)
	Close() error
}

type SSHSession interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	RequestPty(term string, h, w int, modes ssh.TerminalModes) error
	Shell() error
	Wait() error
	Close() error
}

type cryptoSSHDialer struct{}

type sshClient struct {
	client *ssh.Client
}

func (d cryptoSSHDialer) Dial(ctx context.Context, network, address string, config *ssh.ClientConfig) (SSHClient, error) {
	type result struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		client, err := ssh.Dial(network, address, config)
		ch <- result{client: client, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return sshClient{client: res.client}, nil
	}
}

func (c sshClient) NewSession() (SSHSession, error) {
	return c.client.NewSession()
}

func (c sshClient) Close() error {
	return c.client.Close()
}

type CreateRequest struct {
	HostID    string
	CreatedBy string
}

type SessionView struct {
	ID           string                      `json:"id"`
	HostID       string                      `json:"hostId"`
	HostName     string                      `json:"hostName"`
	Status       model.TerminalSessionStatus `json:"status"`
	CreatedAt    string                      `json:"createdAt"`
	WelcomeLines []string                    `json:"welcomeLines"`
}

func NewService(db *gorm.DB, secrets *secret.Service) *Service {
	return &Service{db: db, secrets: secrets, dialer: cryptoSSHDialer{}, timeout: 8 * time.Second}
}

func (s *Service) SetDialer(dialer SSHDialer) {
	if dialer != nil {
		s.dialer = dialer
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*SessionView, error) {
	var host model.Host
	if err := s.db.WithContext(ctx).First(&host, "id = ?", req.HostID).Error; err != nil {
		return nil, err
	}
	session := &model.TerminalSession{
		ID:        uuid.NewString(),
		HostID:    host.ID,
		HostName:  host.Name,
		Status:    model.TerminalSessionStatusConnected,
		CreatedBy: req.CreatedBy,
	}
	if err := s.db.WithContext(ctx).Create(session).Error; err != nil {
		return nil, err
	}
	return view(session), nil
}

func (s *Service) Get(ctx context.Context, id string) (*SessionView, error) {
	var session model.TerminalSession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return view(&session), nil
}

func (s *Service) SessionExists(ctx context.Context, id string) error {
	var session model.TerminalSession
	return s.db.WithContext(ctx).Select("id").First(&session, "id = ?", id).Error
}

func (s *Service) Close(ctx context.Context, id string) error {
	if err := s.SessionExists(ctx, id); err != nil {
		return err
	}
	return s.markDisconnected(ctx, id)
}

func (s *Service) Stream(ctx context.Context, id string, terminal io.ReadWriteCloser) error {
	defer terminal.Close()

	session, host, err := s.sessionHost(ctx, id)
	if err != nil {
		return err
	}
	dialCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	client, err := s.dialHost(dialCtx, &host)
	if err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	defer client.Close()
	_ = s.markConnected(context.Background(), session.ID)

	sshSession, err := client.NewSession()
	if err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	defer sshSession.Close()

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	if err := sshSession.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}
	if err := sshSession.Shell(); err != nil {
		_ = s.markDisconnected(context.Background(), session.ID)
		return err
	}

	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }
	go func() {
		_, _ = io.Copy(stdin, terminal)
		_ = stdin.Close()
		closeDone()
	}()
	go func() {
		_, _ = io.Copy(terminal, stdout)
		closeDone()
	}()
	go func() {
		_, _ = io.Copy(terminal, stderr)
		closeDone()
	}()

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- sshSession.Wait()
		closeDone()
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-waitErr:
	case <-done:
	}
	_ = sshSession.Close()
	_ = client.Close()
	_ = s.markDisconnected(context.Background(), session.ID)
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *Service) sessionHost(ctx context.Context, id string) (*model.TerminalSession, model.Host, error) {
	var session model.TerminalSession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		return nil, model.Host{}, err
	}
	var host model.Host
	if err := s.db.WithContext(ctx).First(&host, "id = ?", session.HostID).Error; err != nil {
		return nil, model.Host{}, err
	}
	return &session, host, nil
}

func (s *Service) dialHost(ctx context.Context, host *model.Host) (SSHClient, error) {
	if s.secrets == nil {
		return nil, errors.New("secret service is not configured")
	}
	value, err := s.secrets.DecryptValue(ctx, host.SSHSecretID)
	if err != nil {
		return nil, err
	}
	secretItem, err := s.secrets.Get(ctx, host.SSHSecretID)
	if err != nil {
		return nil, err
	}
	auth, err := sshAuthMethod(secretItem.Type, value)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            host.SSHUser,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         s.timeout,
	}
	address := net.JoinHostPort(host.Address, fmt.Sprintf("%d", host.SSHPort))
	return s.dialer.Dial(ctx, "tcp", address, config)
}

func (s *Service) markDisconnected(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.TerminalSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    model.TerminalSessionStatusDisconnected,
		"closed_at": &now,
	}).Error
}

func (s *Service) markConnected(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&model.TerminalSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    model.TerminalSessionStatusConnected,
		"closed_at": nil,
	}).Error
}

func sshAuthMethod(secretType model.SecretType, value string) (ssh.AuthMethod, error) {
	switch secretType {
	case model.SecretTypeSSHPassword:
		return ssh.Password(value), nil
	case model.SecretTypeSSHPrivateKey:
		signer, err := ssh.ParsePrivateKey([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("secret type %s cannot be used for ssh", secretType)
	}
}

func view(session *model.TerminalSession) *SessionView {
	return &SessionView{
		ID:        session.ID,
		HostID:    session.HostID,
		HostName:  session.HostName,
		Status:    session.Status,
		CreatedAt: session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		WelcomeLines: []string{
			fmt.Sprintf("AegisOps terminal session %s created.", session.ID),
			"WebSocket shell streaming is ready.",
		},
	}
}
