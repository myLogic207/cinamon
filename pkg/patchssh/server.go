package patchssh

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
	"github.com/myLogic207/gotils/workers"

	"golang.org/x/crypto/ssh"
)

var (
	ErrWorkerPoolInitialized = errors.New("worker pool already initialized")
	defaultServerConfig      = map[string]interface{}{
		"LOGGER": map[string]interface{}{
			"PREFIX":       "SOCKET-SERVER",
			"PREFIXLENGTH": 20,
		},
		"ADDRESS":       "127.0.0.1",
		"PORT":          8080,
		"WORKERS":       100,
		"TIMEOUT":       "5s",
		"KEYFILE":       "id_ed25519",
		"KNOWNHOSTFILE": "known_clients",
		"MAXAUTHTRIES":  3,
		"SERVERVERSION": "SSH-2.0-patchssh",
	}
)

type SocketServer struct {
	logger       log.Logger
	config       config.Config
	sshConfig    *ssh.ServerConfig
	loginManager *AuthManager
	listener     net.Listener
	workerPool   *workers.WorkerPool
}

func NewServer(serverOptions config.Config) (*SocketServer, error) {
	cnf := config.NewConfigWithInitialValues(defaultServerConfig)
	if err := cnf.Merge(serverOptions, true); err != nil {
		return nil, err
	}
	if err := cnf.CompareDefault(defaultServerConfig); err != nil {
		return nil, err
	}

	server := &SocketServer{
		config: cnf,
	}

	loggerConfig, _ := cnf.GetConfig("LOGGER")
	if logger, err := log.NewLogger(loggerConfig); err != nil {
		return nil, err
	} else {
		server.logger = logger
	}
	if err := server.loadSSHConfig(); err != nil {
		return nil, err
	}

	server.logger.Debug(nil, "Server Created")
	return server, nil
}

func (s *SocketServer) loadSSHConfig() error {
	keyFile, _ := s.config.GetString("KEYFILE")
	hostFile, _ := s.config.GetString("KNOWNHOSTFILE")
	maxTries, _ := s.config.GetInt("MAXAUTHTRIES")
	version, _ := s.config.GetString("SERVERVERSION")
	var err error
	s.loginManager, err = NewAuthManager(keyFile, hostFile)
	if err != nil {
		return err
	}
	sshConfig := &ssh.ServerConfig{
		NoClientAuth:                true,
		MaxAuthTries:                maxTries,
		ServerVersion:               version,
		AuthLogCallback:             s.AuthLogCallback,
		PublicKeyCallback:           s.loginManager.PublicKeyCallback,
		NoClientAuthCallback:        s.loginManager.NoAuthCallback,
		PasswordCallback:            s.loginManager.PasswordAuth,
		KeyboardInteractiveCallback: s.loginManager.KeyboardInteractiveAuth,
		BannerCallback:              s.loginManager.Banner,
		PublicKeyAuthAlgorithms: []string{
			ssh.KeyAlgoED25519,
			ssh.KeyAlgoRSA,
		},
	}
	s.sshConfig = sshConfig
	s.sshConfig.AddHostKey(s.loginManager.signer)
	return nil
}

func (s *SocketServer) AuthLogCallback(conn ssh.ConnMetadata, method string, err error) {
	if err == nil {
		s.logger.Info(context.Background(), "Connection from '%s' using '%s'", conn.RemoteAddr().String(), method)
	} else {
		s.logger.Error(context.Background(), "Connection error from '%s' using '%s' auth: %s", conn.RemoteAddr().String(), method, err.Error())
	}
}

func (s *SocketServer) initListener(ctx context.Context) error {
	addr, _ := s.config.GetString("ADDRESS")
	port, _ := s.config.GetInt("PORT")
	address := fmt.Sprintf("%s:%d", addr, port)
	timeout, _ := s.config.GetDuration("TIMEOUT")
	listenConfig := net.ListenConfig{
		KeepAlive: timeout - (timeout / 10),
		Control:   nil,
	}
	listener, err := listenConfig.Listen(ctx, "tcp", address)
	if err != nil {
		return err
	}
	s.logger.Info(ctx, "Listening on %s", address)
	s.listener = listener
	return nil

}

func (s *SocketServer) initWorkerPool(ctx context.Context) error {
	if s.workerPool != nil {
		return ErrWorkerPoolInitialized
	}
	s.logger.Debug(ctx, "Initializing worker pool")
	poolSize, _ := s.config.GetInt("WORKERS")
	poolLoggerConfig, _ := s.config.GetConfig("LOGGER")
	if err := poolLoggerConfig.Set("PREFIX", "SERVER-WORKERPOOL", true); err != nil {
		return err
	}
	poolLogger, err := log.NewLogger(poolLoggerConfig)
	if err != nil {
		return err
	}
	pool, err := workers.NewWorkerPool(ctx, poolSize, poolLogger)
	if err != nil {
		return err
	}
	s.workerPool = pool
	s.logger.Info(ctx, "Worker pool initialized")
	return nil
}

// serve starts accepting connections on the socket, is non blocking, reports startup errors directly and is non blocking
func (s *SocketServer) Serve(ctx context.Context) error {
	subCtx, cancel := context.WithCancelCause(ctx)
	if err := s.initWorkerPool(subCtx); err != nil {
		return err
	}
	if err := s.initListener(subCtx); err != nil {
		return err
	}

	connChan := make(chan net.Conn)
	go s.handleConnectionsLoop(subCtx, connChan)
	// handle errors and context cancel
	go func() {
		<-ctx.Done()
		s.logger.Info(ctx, "Server stopping")
		err := ctx.Err()
		if err != nil && err != context.Canceled {
			s.logger.Error(ctx, "reason: %s", err.Error())
		}
		cancel(err)
		if err := s.listener.Close(); err != nil {
			s.logger.Error(ctx, err.Error())
		}
		s.workerPool = nil
	}()
	s.logger.Info(ctx, "Server started")
	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				s.logger.Error(ctx, err.Error())
				cancel(err)
			} else if errors.Is(err, net.ErrClosed) {
				s.logger.Debug(ctx, "Listener closed")
				return
			}
			connChan <- conn
		}
	}()

	return nil
}

func (s *SocketServer) handleConnectionsLoop(ctx context.Context, connChan <-chan net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-connChan:
			s.logger.Debug(ctx, "New connection from %s", conn.RemoteAddr().String())
			wrapper := NewConnTaskWrapper(conn, s.sshConfig, s.logger)
			s.workerPool.Add(ctx, wrapper)
			s.logger.Debug(ctx, "Connection added to worker pool")
		}
	}
}
