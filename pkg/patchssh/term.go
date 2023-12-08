package patchssh

import (
	"context"
	"fmt"
	log "gotils/logger"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type UserShell interface {
	Execute(context.Context, string) ([]byte, error)
}

type TerminalWrapper struct {
	logger        *log.Logger
	userChannel   ssh.Channel
	systemChannel UserShell
	terminal      *term.Terminal
}

func NewTerminalWrapper(logger *log.Logger, userChannel ssh.Channel, system UserShell) *TerminalWrapper {
	return &TerminalWrapper{
		logger:        logger,
		userChannel:   userChannel,
		systemChannel: system,
	}
}

func (tw *TerminalWrapper) Do(ctx context.Context) error {
	defer func() {
		tw.logger.Debug("User shell finished")
		if err := recover(); err != nil {
			tw.logger.Error("Error in user shell: %s", err)
		}
		if err := tw.userChannel.Close(); err != nil {
			tw.logger.Error("Error closing channel: %s", err.Error())
		}
	}()
	tw.terminal = term.NewTerminal(tw.userChannel, "> ")
	tw.terminal.SetSize(80, 24)
	tw.logger.Debug("User shell started")
	if err := tw.defaultLoop(ctx); err != nil {
		tw.logger.Error("Error in default loop: %s", err.Error())
	}
	return nil
}

func (tw *TerminalWrapper) defaultLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := tw.terminal.ReadLine()
			if err != nil && err != io.EOF {
				tw.logger.Error("Error reading from terminal: %s", err.Error())
				continue
			} else if err == io.EOF {
				return nil
			} else if line == "exit" {
				return nil
			} else if line == "" {
				continue
			}

			tw.logger.Debug("Terminal input: %s", line)
			if result, err := tw.systemChannel.Execute(ctx, line); err != nil {
				tw.sendError(err)
			} else if result != nil {
				tw.sendResult(result)
			}

		}
	}
}

func (tw *TerminalWrapper) sendResult(result []byte) {
	// check if ends with newline
	if _, err := tw.userChannel.Write(result); err != nil {
		tw.logger.Error("Error writing to channel: %s", err.Error())
	}
	<-time.After(1 * time.Microsecond)
	if _, err := tw.terminal.Write([]byte("\r\n")); err != nil {
		tw.logger.Error("Error writing to terminal: %s", err.Error())
	}
}

func (tw *TerminalWrapper) sendError(err error) {
	// send error to stderr
	errMsg := fmt.Sprintf("%sError executing command:\r\n\t%s%s", tw.terminal.Escape.Red, err.Error(), tw.terminal.Escape.Reset)
	if _, err := tw.userChannel.Stderr().Write([]byte(errMsg)); err != nil {
		tw.logger.Error("Error writing to stderr: %s", err.Error())
	}
	<-time.After(1 * time.Microsecond)
	if _, err := tw.userChannel.Write([]byte("\r\n")); err != nil {
		tw.logger.Error("Error writing to channel: %s", err.Error())
	}
}
