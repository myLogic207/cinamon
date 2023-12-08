package patchssh

import (
	"context"
	"errors"
	log "gotils/logger"
	"gotils/workers"
	"io"
	"net"

	"golang.org/x/crypto/ssh"
)

type contextKey string

var (
	contextKeyChannelID = contextKey("channel-id")
)

type ChannelHandler func(ctx context.Context, channel ssh.NewChannel) error

type RequestHandler func(ctx context.Context, channel ssh.Channel, request *ssh.Request)

type SubsystemHandler func(ctx context.Context, subsystem string) error

type connTaskWrapper struct {
	workers.Task
	logger    *log.Logger
	sshConfig *ssh.ServerConfig
	conn      net.Conn
	// ChannelHandlers allow overriding the built-in session handlers or provide
	// extensions to the protocol, such as tcpip forwarding. By default only the
	// "session" handler is enabled.
	ChannelHandlers map[string]ChannelHandler

	// RequestHandlers allow overriding the server-level request handlers or
	// provide extensions to the protocol, such as tcpip forwarding. By default
	// no handlers are enabled.
	RequestHandlers map[string]RequestHandler

	// SubsystemHandlers are handlers which are similar to the usual SSH command
	// handlers, but handle named subsystems.
	SubsystemHandlers map[string]SubsystemHandler

	// per default users need a shell after a shell request
	ShellHandler UserShell
}

func NewConnTaskWrapper(conn net.Conn, sshConfig *ssh.ServerConfig, logger *log.Logger) *connTaskWrapper {
	wrapper := &connTaskWrapper{
		logger:    logger,
		conn:      conn,
		sshConfig: sshConfig,
	}
	wrapper.ChannelHandlers = map[string]ChannelHandler{
		"session": wrapper.DefaultSessionHandler,
	}
	wrapper.RequestHandlers = map[string]RequestHandler{
		"default": wrapper.DefaultRequestHandler,
		"shell":   wrapper.ShellRequestHandler,
		"pty-req": wrapper.TerminalRequestHandler,
	}
	return wrapper
}

func (cw *connTaskWrapper) OnFinish() {
	// workernode exited normally
	if err := cw.conn.Close(); err != nil {
		cw.logger.Error(err.Error())
	}
}

func (cw *connTaskWrapper) OnError(err error) {
	// workernode exited with error
	if err := cw.conn.Close(); err != nil {
		cw.logger.Error(err.Error())
	}
}

func (cw *connTaskWrapper) Do(ctx context.Context) error {
	// handle connection
	// perform ssh handshake
	cw.logger.Debug("Performing ssh handshake")
	sshConn, chans, reqs, err := ssh.NewServerConn(cw.conn, cw.sshConfig)
	if err != nil {
		return err
	}
	cw.logger.Debug("Connection from %s established", sshConn.RemoteAddr().String())
	// handle ssh connection
	// handle ssh channel requests
	go cw.handleChannels(ctx, chans)

	// handle ssh global requests
	go ssh.DiscardRequests(reqs)

	cw.logger.Info("Connection %s established", sshConn.RemoteAddr().String())
	// block until ssh connection is finished
	if err := sshConn.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return sshConn.Close()
}

func (cw *connTaskWrapper) handleChannels(ctx context.Context, chans <-chan ssh.NewChannel) {
	chanCounter := 0
	for newChannel := range chans {
		chanCtx := context.WithValue(ctx, contextKeyChannelID, chanCounter)
		cw.handleChannel(chanCtx, newChannel)
	}
}

func (cw *connTaskWrapper) handleChannel(ctx context.Context, newChannel ssh.NewChannel) {
	handler, ok := cw.ChannelHandlers[newChannel.ChannelType()]
	if !ok {
		newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		return
	}
	go handler(ctx, newChannel)
}

func (cw *connTaskWrapper) DefaultSessionHandler(ctx context.Context, channel ssh.NewChannel) error {
	newChan, request, err := channel.Accept()
	if err != nil {
		return err
	}
	for req := range request {
		requestHandler, ok := cw.RequestHandlers[req.Type]
		if !ok {
			requestHandler = cw.RequestHandlers["default"]
		}
		go requestHandler(ctx, newChan, req)
	}
	return nil
}

func (cw *connTaskWrapper) DefaultRequestHandler(ctx context.Context, channel ssh.Channel, request *ssh.Request) {
	reply := false
	result := []byte{}

	cw.logger.Debug("Request type: %s", request.Type)
	cw.logger.Debug("Request payload: %v", request.Payload)

	request.Reply(reply, result)
}

func (cw *connTaskWrapper) ShellRequestHandler(ctx context.Context, channel ssh.Channel, request *ssh.Request) {
	// prepare shell wrapper
	cw.ShellHandler = NewShellWrapper(cw.logger)
	request.Reply(true, nil)
}

func (cw *connTaskWrapper) TerminalRequestHandler(ctx context.Context, channel ssh.Channel, request *ssh.Request) {
	if cw.ShellHandler == nil {
		// no shell handler available
		request.Reply(false, nil)
		return
	}
	// prepare terminal wrapper
	terminal := NewTerminalWrapper(cw.logger, channel, cw.ShellHandler)
	go terminal.Do(ctx)
	if request.WantReply {
		request.Reply(true, nil)
	}
}
