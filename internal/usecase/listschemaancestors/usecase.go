package listschemaancestors

import (
	"context"

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
	slug string,
	version int,
) ([]domain.ID, error) {
	return u.repository.Ancestors(ctx, domain.ID{Slug: slug, Version: version})
}
