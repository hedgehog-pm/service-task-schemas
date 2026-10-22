package findschemas

import (
	"context"
	"fmt"
	"slices"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type Usecase struct {
	repository repository
}

func New(repository repository) *Usecase {
	return &Usecase{repository: repository}
}

func (u *Usecase) Do(
	ctx context.Context,
	fields []string,
) ([]domain.Schema, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf(
			"%w: at least one field is required",
			domain.ErrInvalidInput,
		)
	}

	if slices.Contains(fields, "") {
		return nil, fmt.Errorf(
			"%w: field name must not be empty",
			domain.ErrInvalidInput,
		)
	}

	return u.repository.FindSchemasByFields(ctx, fields)
}
