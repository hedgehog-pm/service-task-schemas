package listschemaancestors

import (
	"context"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type repository interface {
	Ancestors(context.Context, domain.ID) ([]domain.ID, error)
}
