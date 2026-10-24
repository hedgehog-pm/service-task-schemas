package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
	sqlcgen "github.com/hedgehog-pm/service-task-schemas/internal/generated/sqlc/schema"
)

type Postgres struct {
	db      *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db, queries: sqlcgen.New(db)}
}

func (r *Postgres) Get(
	ctx context.Context,
	id domain.ID,
) (json.RawMessage, error) {
	document, err := r.queries.GetEffectiveSchema(
		ctx,
		sqlcgen.GetEffectiveSchemaParams{
			Slug: id.Slug, Version: int32(id.Version),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, id)
	}

	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", id, err)
	}

	if len(document) == 0 {
		return nil, fmt.Errorf("schema %s has no effective schema", id)
	}

	return document, nil
}

func (r *Postgres) Ancestors(
	ctx context.Context,
	id domain.ID,
) ([]domain.ID, error) {
	rows, err := r.queries.GetSchemaLineage(ctx, sqlcgen.GetSchemaLineageParams{
		Slug: id.Slug, Version: int64(id.Version),
	})
	if err != nil {
		return nil, fmt.Errorf("read schema ancestors %s: %w", id, err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, id)
	}

	ancestors := make([]domain.ID, len(rows))
	for i, row := range rows {
		ancestors[i] = domain.ID{Slug: row.Slug, Version: int(row.Version)}
	}

	return ancestors, nil
}

func (r *Postgres) Versions(ctx context.Context, slug string) ([]int, error) {
	stored, err := r.queries.ListSchemaVersions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("list versions for %q: %w", slug, err)
	}

	versions := make([]int, len(stored))
	for i, version := range stored {
		versions[i] = int(version)
	}

	return versions, nil
}

// Find возвращает найденные модели; отсутствующие ID в результат не входят.
func (r *Postgres) Find(
	ctx context.Context,
	ids []domain.ID,
) ([]domain.Schema, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	params := sqlcgen.FindSchemasParams{
		Slugs:    make([]string, len(ids)),
		Versions: make([]int64, len(ids)),
	}
	for i, id := range ids {
		params.Slugs[i] = id.Slug
		params.Versions[i] = int64(id.Version)
	}

	rows, err := r.queries.FindSchemas(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("read schemas: %w", err)
	}

	result := make([]domain.Schema, len(rows))
	for i, row := range rows {
		result[i] = SchemaToDomain(row)
	}

	return result, nil
}

func (r *Postgres) FindSchemasByFields(
	ctx context.Context,
	fields []string,
) ([]domain.Schema, error) {
	rows, err := r.queries.FindSchemasByFields(ctx, fields)
	if err != nil {
		return nil, fmt.Errorf("find schemas with all fields: %w", err)
	}

	result := make([]domain.Schema, len(rows))
	for i, row := range rows {
		result[i] = SchemaToDomain(row)
	}

	return result, nil
}

// Insert атомарно записывает новые схемы.
func (r *Postgres) Insert(ctx context.Context, models []domain.Schema) error {
	if len(models) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	for _, model := range models {
		inserted, err := queries.InsertSchema(
			ctx,
			DomainToInsertSchemaParams(model),
		)
		if err != nil {
			return fmt.Errorf("insert schema %s: %w", model.ID, err)
		}

		if inserted == 0 {
			// Версию опубликовали параллельно после чтения в usecase.
			return fmt.Errorf(
				"schema %s already exists, retry publish",
				model.ID,
			)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

type txKey struct{}

func (r *Postgres) InTx(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}
