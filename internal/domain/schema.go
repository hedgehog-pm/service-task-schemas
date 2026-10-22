// Package domain собирает итоговые json-схемы используя наследование.
package domain

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type ID struct {
	Slug    string `json:"slug"    validate:"required"`
	Version int    `json:"version" validate:"gte=1"`
}

// String возвращает ID в формате <slug>.v<version>.
func (id ID) String() string {
	return fmt.Sprintf("%s.v%d", id.Slug, id.Version)
}

type Schema struct {
	ID ID

	// Parent - родительская схема. nil означает корневую схему.
	Parent *ID

	// OwnSchema только поля, которые были добавлены текущей (дочерней) схемой.
	OwnSchema json.RawMessage

	// EffectiveSchema итоговая схема, со всеми полями, включая родительские.
	// Пустое значение означает, что схема ещё не собрана.
	EffectiveSchema json.RawMessage
}

// IsComposed сообщает, была ли собрана схема.
func (s Schema) IsComposed() bool {
	return len(s.EffectiveSchema) != 0
}

// RequiresParent сообщает, нужен ли родитель для сборки схемы.
// Готовая схема уже содержит унаследованные правила.
func (s Schema) RequiresParent() bool {
	return s.Parent != nil && !s.IsComposed()
}

func (s Schema) SameDefinition(other Schema) bool {
	if !reflect.DeepEqual(s.Parent, other.Parent) {
		return false
	}

	var a, b any
	if json.Unmarshal(s.OwnSchema, &a) != nil {
		return false
	}

	if json.Unmarshal(other.OwnSchema, &b) != nil {
		return false
	}

	return reflect.DeepEqual(a, b)
}
