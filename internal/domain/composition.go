package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/samber/lo"
	definitionvalidator "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/hedgehog-pm/service-task-schemas/internal/pkg/topsort"
)

// ComposeSchemas собирает дерево схем, наследуя родительские поля.
// Cборка происходит по принципу:
//
//	models[i].EffectiveSchema = [Schema.Parent.EffectiveSchema] + [Schema.OwnSchema]
//
// Схема с непустым EffectiveSchema считается готовой, и её предки не нужны.
// При ошибке частичный результат не возвращается.
//
// Ошибки, вызванные входными данными, оборачивают ErrInvalidInput.
// Остальные ошибки означают сбой или нарушенный инвариант.
func ComposeSchemas(models []Schema) ([]Schema, error) {
	var problems []error

	for _, model := range models {
		if err := validate.Struct(model); err != nil {
			problems = append(
				problems,
				fmt.Errorf("schema %s: %w", model.ID, err),
			)
		}
	}

	if err := errors.Join(problems...); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	ordered, err := topsort.Sort(models,
		func(s Schema) ID {
			return s.ID
		},
		func(s Schema) []ID {
			if !s.RequiresParent() {
				return nil
			}

			return []ID{lo.FromPtr(s.Parent)}
		},
	)
	if err != nil {
		// Цикл в наследовании или отсутствующий родитель.
		return nil, fmt.Errorf(
			"schema inheritance: %w: %w",
			ErrInvalidInput,
			err,
		)
	}

	definition, err := compileDefinition()
	if err != nil {
		return nil, fmt.Errorf("compile definition meta-schema: %w", err)
	}

	prepared := make(map[ID]Schema, len(ordered))

	for i, model := range ordered {
		if model.IsComposed() {
			prepared[model.ID] = model
			continue
		}

		parent, ok := prepared[lo.FromPtr(model.Parent)]
		if model.RequiresParent() && !ok {
			// Ошибка сборки родителя уже записана, поэтому его потомков пропускаем.
			continue
		}

		model.EffectiveSchema, err = compose(model, parent, definition)
		if err != nil {
			problems = append(
				problems,
				fmt.Errorf("schema %s: %w", model.ID, err),
			)

			continue
		}

		ordered[i] = model
		prepared[model.ID] = model
	}

	// Класс ошибки проставлен внутри compose точечно: в пачке могут быть
	// и ошибки данных, и сбои, поэтому целиком её не оборачиваем.
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}

	return ordered, nil
}

func compose(
	model, parent Schema,
	definition *definitionvalidator.Schema,
) (json.RawMessage, error) {
	var document any
	if err := json.Unmarshal(model.OwnSchema, &document); err != nil {
		return nil, fmt.Errorf(
			"%w: decode definition: %w",
			ErrInvalidInput,
			err,
		)
	}

	if err := definition.Validate(document); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	var own jsonschema.Schema
	if err := json.Unmarshal(model.OwnSchema, &own); err != nil {
		return nil, fmt.Errorf(
			"%w: decode definition: %w",
			ErrInvalidInput,
			err,
		)
	}

	if own.ID == "" {
		// Собственный $id сохраняет область локальных $ref при включении в allOf.
		own.ID = "urn:schemas:part:" + url.PathEscape(model.ID.String())
	}

	var inherited jsonschema.Schema

	if model.RequiresParent() {
		// Родитель уже собран и прошёл проверки, поэтому ошибки ниже
		// означают повреждённые данные или баг, а не неверный ввод.
		if err := json.Unmarshal(
			parent.EffectiveSchema,
			&inherited,
		); err != nil {
			return nil, fmt.Errorf("decode parent %s: %w", parent.ID, err)
		}

		if len(inherited.AllOf) == 0 {
			return nil, fmt.Errorf("parent %s has no composed parts", parent.ID)
		}
	}

	// allOf допускает повторные ограничения поля, но наше наследование запрещает их.
	for _, ancestor := range inherited.AllOf {
		if ancestor == nil {
			return nil, fmt.Errorf(
				"parent %s has a null schema part",
				parent.ID,
			)
		}

		for name := range own.Properties {
			if _, exists := ancestor.Properties[name]; exists {
				return nil, fmt.Errorf(
					"%w: property %q overrides an ancestor",
					ErrInvalidInput,
					name,
				)
			}
		}
	}

	// Берём открытые части родителя, иначе его закрытая оболочка отвергла бы новые поля.
	// Закрываем только итоговую схему, потому что Not от пустой схемы сериализуется как false.
	effective := &jsonschema.Schema{
		Schema:                dialect,
		ID:                    "urn:schemas:compiled",
		AllOf:                 append(inherited.AllOf, &own),
		UnevaluatedProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}

	// Resolve без загрузчика отвергает внешние ссылки, а контракт должен быть автономным.
	if _, err := effective.Resolve(nil); err != nil {
		return nil, fmt.Errorf(
			"%w: resolve effective schema: %w",
			ErrInvalidInput,
			err,
		)
	}

	return json.Marshal(effective)
}
