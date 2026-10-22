package findschemas

import (
	"context"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type repository interface {
	FindSchemasByFields(
		ctx context.Context,
		fields []string,
	) ([]domain.Schema, error)
}
