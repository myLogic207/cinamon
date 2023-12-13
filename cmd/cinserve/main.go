package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"

	"github.com/myLogic207/cinnamon/internal/dbconnect"
	"github.com/myLogic207/cinnamon/internal/models"
	ssh "github.com/myLogic207/cinnamon/patchssh"
)

const (
	ENV_PREFIX    = "CINNAMON"
	CANCEL_BUFFER = 10
	END_TIMEOUT   = 1 * time.Second
)

var (
	defaultConfig = map[string]interface{}{
		"WORKDIR": "work",
		"LOGGER": map[string]interface{}{
			"PREFIX":       "CINNAMON",
			"PREFIXLENGTH": 20,
			"LEVEL":        "DEBUG",
			"WRITERS": map[string]interface{}{
				"STDOUT": true,
				"FILE": map[string]interface{}{
					"ACTIVE":   true,
					"MAXSIZE":  10,
					"FOLDER":   "logs",
					"ROTATING": true,
				},
			},
		},
		"SERVER": map[string]interface{}{
			"ADDRESS": "127.0.0.1",
			"PORT":    2222,
			"WORKERS": 3,
			"LOGGER": map[string]interface{}{
				"PREFIX": "CINNAMON-SERVER",
				"WRITERS": map[string]interface{}{
					"STDOUT": true,
					"FILE": map[string]interface{}{
						"ACTIVE": true,
						"FOLDER": "logs",
					},
				},
			},
			"KEYFILE":       "ssh/server_key",
			"KNOWNHOSTFILE": "ssh/known_clients",
		},
		"DB": map[string]interface{}{
			"TYPE":     "postgres",
			"HOST":     "localhost",
			"PORT":     "5432",
			"USERNAME": "postgres",
			"PASSWORD": "postgres",
			"NAME":     "postgres",
			"SSLMODE":  "disable",
			"TIMEZONE": "Europe/Berlin",
			"POOL":     map[string]interface{}{},
			"LOGGER": map[string]interface{}{
				"PREFIX": "CINNAMON-DB",
				"WRITERS": map[string]interface{}{
					"STDOUT": true,
					"FILE": map[string]interface{}{
						"ACTIVE": true,
						"FOLDER": "logs",
					},
				},
			},
			"CACHE": map[string]interface{}{
				"ACTIVE": false,
			},
		},
	}
)

func main() {
	mainCtx, mainCancel := context.WithCancelCause(context.Background())

	masterConfig, err := prep(mainCtx)
	if err != nil {
		mainCancel(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for sig := range c {
			mainCancel(errors.New(sig.String()))
		}
	}()

	defer shutdown(mainCtx)
	if err := run(mainCtx, masterConfig); err != nil {
		panic(err)
	}
}

func prep(ctx context.Context) (config.Config, error) {
	options, err := config.LoadConfig(ctx, []string{ENV_PREFIX}, []string{}, false)
	if err != nil {
		return nil, err
	}
	masterConfig := config.NewWithInitialValues(defaultConfig)
	if err := masterConfig.Merge(options, true); err != nil {
		return nil, err
	}
	if err := masterConfig.CompareDefault(defaultConfig); err != nil {
		return nil, err
	}

	workdir, _ := masterConfig.GetString("WORKDIR")
	if stat, err := os.Stat(workdir); err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(workdir, 0755); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if !stat.IsDir() {
		return nil, errors.New("workdir is not a directory")
	}
	// change working directory
	if err := os.Chdir(workdir); err != nil {
		return nil, err
	}

	return masterConfig, nil
}

func run(ctx context.Context, masterConfig config.Config) error {
	loggerConfig, _ := masterConfig.GetConfig("LOGGER")
	logger, err := log.NewLogger(loggerConfig)
	if err != nil {
		return err
	}
	logger.Info(ctx, "Logger initialized")

	dbConfig, _ := masterConfig.GetConfig("DB")
	db, err := dbconnect.NewDB(dbConfig)
	if err != nil {
		return err
	}
	logger.Info(ctx, "Database initialized")

	// userDB, err := models.NewUserDB(db)
	// if err != nil {
	// 	return err
	// }
	// logger.Info(ctx, "UserDB initialized")

	keyDB, err := models.NewKeyDB(db)
	if err != nil {
		return err
	}
	logger.Info(ctx, "KeyDB initialized")

	serverConfig, _ := masterConfig.GetConfig("SERVER")
	server, err := ssh.NewServer(serverConfig, keyDB)
	if err != nil {
		return err
	}
	logger.Info(ctx, "Server initialized")
	if err := server.Serve(ctx); err != nil {
		return err
	}
	logger.Info(ctx, "Server started")

	// wait for context to be done/run indefinitely
	<-ctx.Done()
	if err := ctx.Err(); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func shutdown(ctx context.Context) {
	println("Server received shutdown signal")
	if rec := recover(); rec != nil {
		println("panic recovered: %s (%v)", rec)
	}
	// wait for all workers to finish etc.
	<-time.After(END_TIMEOUT)
	if err := ctx.Err(); err != nil && err != context.Canceled {
		println("Reason: %v", err)
	}
	println("Server stopped")
	os.Exit(0)
}
