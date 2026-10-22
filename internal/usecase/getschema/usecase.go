// Package getschema возвращает итоговую схему по точной паре slug и version.
package getschema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type Usecase struct {
	repository repository
}

func New(repository repository) *Usecase {
	return &Usecase{repository: repository}
}

// Do возвращает итоговую схему по slug и version.
// Возвращает [domain.ErrInvalidInput] при version < 1
// и [domain.ErrNotFound], если схемы нет.
func (u *Usecase) Do(
	ctx context.Context,
	slug string,
	version int,
) (json.RawMessage, error) {
	if version < 1 {
		return nil, fmt.Errorf(
			"%w: version must be at least 1",
			domain.ErrInvalidInput,
		)
	}

	id := domain.ID{Slug: slug, Version: version}

	return u.repository.Get(ctx, id)
}
