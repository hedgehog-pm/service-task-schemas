package publishschemas

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

type Usecase struct {
	repository repository
	compiler   compiler
}

func New(repository repository, compiler compiler) *Usecase {
	return &Usecase{repository: repository, compiler: compiler}
}

// Do собирает итоговые схемы и сохраняет их в хранилище, не изменяя входные схемы.
func (u *Usecase) Do(ctx context.Context, models []domain.Schema) error {
	if len(models) == 0 {
		return nil
	}

	return u.repository.InTx(ctx, func(ctx context.Context) error {
		return u.publish(ctx, models)
	})
}

func (u *Usecase) publish(ctx context.Context, models []domain.Schema) error {
	// Ловим дубликаты в models и собираем ids которые надо
	// запросить из хранилища.
	ids := lo.Map(models, func(model domain.Schema, _ int) domain.ID {
		return model.ID
	})
	if duplicates := lo.FindDuplicates(ids); len(duplicates) > 0 {
		return fmt.Errorf("duplicate schema %s", duplicates[0])
	}

	requested := lo.Keyify(ids)

	// Родителей вне батча тоже читаем из хранилища.
	ids = append(ids, lo.FilterMap(
		models,
		func(model domain.Schema, _ int) (domain.ID, bool) {
			parent := lo.FromPtr(model.Parent)
			_, inBatch := requested[parent]

			return parent, model.RequiresParent() && !inBatch
		},
	)...)

	// Ищем ранее опубликованные схемы из models, и
	// родительские схемы из models[i].Parent
	found, err := u.repository.Find(ctx, ids)
	if err != nil {
		return err
	}

	stored := lo.KeyBy(found, func(schema domain.Schema) domain.ID {
		return schema.ID
	})

	// Опубликованная версия неизменяема
	fresh := make(map[domain.ID]struct{}, len(models))
	prepared := make([]domain.Schema, 0, len(models)+len(found))

	for _, model := range models {
		saved, exists := stored[model.ID]
		if !exists {
			// не доверяем тому, что лежит в models
			model.EffectiveSchema = nil
			prepared = append(prepared, model)
			fresh[model.ID] = struct{}{}

			continue
		}

		if !model.SameDefinition(saved) {
			return fmt.Errorf(
				"schema %s already exists with another definition",
				model.ID,
			)
		}
	}

	if len(fresh) == 0 {
		return nil
	}

	// found содержит и уже сохранённые версии батча, и родителей.
	ordered, err := domain.ComposeSchemas(
		slices.Concat(prepared, found),
		u.compiler,
	)
	if err != nil {
		return err
	}

	batch := lo.Filter(ordered, func(model domain.Schema, _ int) bool {
		_, ok := fresh[model.ID]

		return ok
	})

	return u.repository.Insert(ctx, batch)
}
