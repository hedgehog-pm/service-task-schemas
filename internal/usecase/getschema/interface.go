package getschema

import (
	"context"
	"encoding/json"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type repository interface {
	Get(ctx context.Context, id domain.ID) (json.RawMessage, error)
}
