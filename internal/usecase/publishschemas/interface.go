package publishschemas

import (
	"context"
	"encoding/json"

	"github.com/hedgehog-pm/schema"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type repository interface {
	InTx(context.Context, func(context.Context) error) error
	Find(context.Context, []domain.ID) ([]domain.Schema, error)
	Insert(context.Context, []domain.Schema) error
}

type compiler interface {
	Compile(json.RawMessage) (*schema.Schema, error)
}
