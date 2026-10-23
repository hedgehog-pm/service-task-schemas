package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hedgehog-pm/schema"
	"github.com/samber/lo"

	"github.com/hedgehog-pm/service-task-schemas/internal/pkg/topsort"
)

type compiler interface {
	Compile(json.RawMessage) (*schema.Schema, error)
}

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
func ComposeSchemas(models []Schema, compiler compiler) ([]Schema, error) {
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

	definition, err := compiler.Compile(definitionSchema)
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

		model.EffectiveSchema, err = compose(
			model,
			parent,
			definition,
			compiler,
		)
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
	definition *schema.Schema,
	compiler compiler,
) (json.RawMessage, error) {
	if err := definition.Validate(model.OwnSchema); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	// TODO: в данный момент поле x-extensions может содержать какие угодно поля.
	// На значение этих полей планируется завязывать бизнес-логику, если она будет
	// опираться конкретный тип из x-extensions, а в одной из схем допустят опечатку,
	// то логика может сломаться. Если потребуется, то надо подумать как это решить.
	// Потенциально, можно завести их в виде схемы, и запретить менять типы при оверрайде.
	source, extensions, err := splitExtensions(model.OwnSchema)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: extract extensions: %w",
			ErrInvalidInput,
			err,
		)
	}

	var own jsonschema.Schema
	if err := json.Unmarshal(source, &own); err != nil {
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

	var inherited struct {
		AllOf []*jsonschema.Schema `json:"allOf"`
	}

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

	if extensions != nil {
		effective.Extra = map[string]any{"x-extensions": extensions}
	}

	document, err := json.Marshal(effective)
	if err != nil {
		return nil, fmt.Errorf("encode effective schema: %w", err)
	}

	// Проверяем собранный контракт целиком, включая ссылки между его частями.
	if _, err := compiler.Compile(document); err != nil {
		return nil, fmt.Errorf(
			"%w: compile effective schema: %w",
			ErrInvalidInput,
			err,
		)
	}

	return document, nil
}

func splitExtensions(
	document json.RawMessage,
) (json.RawMessage, json.RawMessage, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(document), []byte("{")) {
		return document, nil, nil
	}

	var keywords map[string]json.RawMessage
	if err := json.Unmarshal(document, &keywords); err != nil {
		return nil, nil, err
	}

	extensions := keywords["x-extensions"]
	if extensions == nil {
		return document, nil, nil
	}

	delete(keywords, "x-extensions")

	source, err := json.Marshal(keywords)
	if err != nil {
		return nil, nil, err
	}

	return source, extensions, nil
}
