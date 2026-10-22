package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/hedgehog-pm/service-task-schemas/internal/repository/schema"
	"github.com/hedgehog-pm/service-task-schemas/internal/seed"
	"github.com/hedgehog-pm/service-task-schemas/internal/usecase/publishschemas"
	"github.com/hedgehog-pm/service-task-schemas/schemas"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn := flag.String(
		"dsn",
		os.Getenv("DATABASE_URL"),
		"PostgreSQL connection string",
	)
	migrations := flag.String(
		"migrations",
		"migrations/postgresql/master",
		"goose migrations directory",
	)

	flag.Parse()

	if *dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db, *migrations); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	models, err := seed.LoadFS(schemas.FS)
	if err != nil {
		return fmt.Errorf("load seed files: %w", err)
	}

	publisher := publishschemas.New(schema.NewPostgres(pool))

	if err := publisher.Do(ctx, models); err != nil {
		return fmt.Errorf("insert seed schemas: %w", err)
	}

	slog.InfoContext(ctx, "migrations applied", "schema_count", len(models))

	return nil
}
