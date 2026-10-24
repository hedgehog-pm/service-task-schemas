package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"

	"github.com/hedgehog-pm/service-task-schemas/internal/api"
	generatedapi "github.com/hedgehog-pm/service-task-schemas/internal/generated/api"
	"github.com/hedgehog-pm/service-task-schemas/internal/repository/schema"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/findschemas"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/getschema"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/listschemaancestors"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/listschemaversions"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "service failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	repository := schema.NewPostgres(db)
	handler := api.NewServer(
		getschema.New(repository),
		listschemaversions.New(repository),
		findschemas.New(repository),
		listschemaancestors.New(repository),
	)

	server := &http.Server{
		Addr:              listenAddress(),
		Handler:           newEchoServer(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	listenConfig := net.ListenConfig{}

	listener, err := listenConfig.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	slog.InfoContext(
		ctx,
		"http server started",
		"address",
		listener.Addr().String(),
	)

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}

		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}

func newEchoServer(handler generatedapi.ServerInterface) *echo.Echo {
	server := echo.New()
	server.HTTPErrorHandler = api.HTTPErrorHandler
	generatedapi.RegisterHandlers(server, handler)

	return server
}

func listenAddress() string {
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		return addr
	}

	return ":8080"
}
