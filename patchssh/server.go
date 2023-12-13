package patchssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"net"

	"github.com/myLogic207/cinnamon/internal/models"
	"github.com/myLogic207/cinnamon/patchssh/auth"
	"github.com/myLogic207/cinnamon/patchssh/ui"
	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
	"github.com/myLogic207/gotils/workers"

	"golang.org/x/crypto/ssh"
)

var defaultServerConfig = map[string]interface{}{
	"LOGGER": map[string]interface{}{
		"PREFIX":       "SOCKET-SERVER",
		"PREFIXLENGTH": 20,
	},
	"ADDRESS": "127.0.0.1",
	"PORT":    8080,
	"WORKERS": 100,
	"TIMEOUT": "5s",
	// if key is not present, default key is used or new key is generated
	// "HOSTKEY":           "",
	"MAXAUTHTRIES":  3,
	"SERVERVERSION": "SSH-2.0-patchssh",
}

type SocketServer struct {
	logger       log.Logger
	config       config.Config
	sshConfig    *ssh.ServerConfig
	loginManager *auth.AuthManager
	listener     net.Listener
	workerPool   *workers.WorkerPool
}

func NewServer(serverOptions config.Config, keyDB models.KeyDB) (*SocketServer, error) {
	cnf := config.NewWithInitialValues(defaultServerConfig)
	if err := cnf.Merge(serverOptions, true); err != nil {
		return nil, err
	}
	if err := cnf.CompareDefault(defaultServerConfig); err != nil {
		return nil, err
	}

	if keyDB == nil {
		return nil, ErrMissingDBConn
	}

	loggerConfig, _ := cnf.GetConfig("LOGGER")
	logger, err := log.NewLogger(loggerConfig)
	if err != nil {
		return nil, err
	}

	server := &SocketServer{
		config:       cnf,
		logger:       logger,
		loginManager: auth.NewAuthManager(keyDB),
	}

	return server, nil
}

func (s *SocketServer) ensureHostKey(ctx context.Context) ([]byte, error) {
	// get key from config
	pemBytes := []byte{}
	configSet := false
	if keyString, err := s.config.GetString("HOSTKEY"); err == nil {
		configSet = true
		rawPemBytes, _ := pem.Decode([]byte(keyString))
		if rawPemBytes != nil {
			pemBytes = pem.EncodeToMemory(rawPemBytes)
		}
	} else {
		// key not set in config, generate new key
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		pemKey, err := ssh.MarshalPrivateKey(privateKey, "")
		if err != nil {
			return nil, err
		}
		pemBytes = pem.EncodeToMemory(pemKey)
	}

	// get current key from db
	dbKey, err := s.loginManager.GetHostKey(ctx)
	if err != nil && !errors.Is(err, models.ErrKeyNotFound) {
		return nil, err
	}

	if errors.Is(err, models.ErrKeyNotFound) || (configSet && err == nil && !bytes.Equal(dbKey, pemBytes)) {
		// key not set in db write key to db
		// or key in db, and not equal with config
		if err := s.loginManager.SetHostKey(ctx, pemBytes); err != nil {
			return nil, err
		}
	}

	return pemBytes, nil
}

func (s *SocketServer) loadSSHConfig(ctx context.Context) error {
	maxTries, _ := s.config.GetInt("MAXAUTHTRIES")
	version, _ := s.config.GetString("SERVERVERSION")
	sshConfig := &ssh.ServerConfig{
		NoClientAuth:         false,
		MaxAuthTries:         maxTries,
		ServerVersion:        version,
		AuthLogCallback:      s.AuthLogCallback,
		PublicKeyCallback:    s.loginManager.PublicKeyCallback,
		NoClientAuthCallback: s.loginManager.NoAuthCallback,
		PasswordCallback:     s.loginManager.PasswordAuth,
		// KeyboardInteractiveCallback: s.loginManager.KeyboardInteractiveAuth,
		BannerCallback: ui.Banner,
		PublicKeyAuthAlgorithms: []string{
			ssh.KeyAlgoED25519,
			ssh.KeyAlgoRSA,
		},
	}
	localKey, err := s.ensureHostKey(ctx)
	if err != nil {
		s.logger.Error(ctx, err.Error())
		return err
	}

	signer, err := ssh.ParsePrivateKey(localKey)
	if err != nil {
		return err
	}

	sshConfig.AddHostKey(signer)
	s.sshConfig = sshConfig
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
		return ErrWorkerPoolAlreadyInit
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
	if err := s.loadSSHConfig(ctx); err != nil {
		s.logger.Error(ctx, err.Error())
		return ErrSSHConfigReason{err}
	}

	if err := s.initWorkerPool(ctx); err != nil {
		return ErrInitWorkerPoolReason{err}
	}
	if err := s.initListener(ctx); err != nil {
		return err
	}

	connChan := make(chan net.Conn)
	go s.handleConnectionsLoop(ctx, connChan)
	// handle errors and context cancel
	quitChan := make(chan error)
	go func() {
		<-ctx.Done()
		s.logger.Info(ctx, "Server stopping")
		err := ctx.Err()
		if err != nil && err != context.Canceled {
			s.logger.Error(ctx, "reason: %s", err.Error())
		}
		if err := s.listener.Close(); err != nil {
			s.logger.Error(ctx, err.Error())
		}
		if err := <-quitChan; err != nil && !errors.Is(err, net.ErrClosed) {
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
				quitChan <- err
				return
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
