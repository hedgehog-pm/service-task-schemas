// Package listschemaversions возвращает версии схемы по slug.
package listschemaversions

import (
	"context"
	"fmt"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type Usecase struct {
	repository repository
}

func New(repository repository) *Usecase {
	return &Usecase{repository: repository}
}

// Do возвращает версии схемы по slug.
// Если версий нет, возвращает [domain.ErrNotFound].
func (u *Usecase) Do(ctx context.Context, slug string) ([]int, error) {
	versions, err := u.repository.Versions(ctx, slug)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("%w: slug %q", domain.ErrNotFound, slug)
	}

	return versions, nil
}
