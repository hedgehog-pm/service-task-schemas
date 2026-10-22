// Package seed читает seed-файлы схем, проверяет их оболочку и возвращает
// модели для записи в хранилище.
package seed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/hedgehog-pm/service-task-schemas/internal/domain"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type file struct {
	domain.ID

	Parent *domain.ID      `json:"parent,omitempty" validate:"omitempty"`
	Schema json.RawMessage `json:"schema"           validate:"required"`
}

// LoadFS читает json-файлы <slug>.v<version>.json из fsys, проверяет
// совпадение имени с содержимым и пропускает подкаталоги и не-json-файлы.
// При ошибках возвращает nil и составную ошибку.
func LoadFS(fsys fs.FS) ([]domain.Schema, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read schemas directory: %w", err)
	}

	documents := make([]json.RawMessage, 0, len(entries))
	filenames := make([]string, 0, len(entries))

	var problems []error

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := entry.Name()

		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, fmt.Errorf("read %q: %w", name, err))

			continue
		}

		documents = append(documents, content)
		filenames = append(filenames, name)
	}

	if err := errors.Join(problems...); err != nil {
		return nil, err
	}

	models, err := LoadJSON(documents)
	if err != nil {
		return nil, fmt.Errorf("load schema JSON: %w", err)
	}

	for index, model := range models {
		filename := filenames[index]

		expected := model.ID.String() + ".json"
		if filename != expected {
			problems = append(problems, fmt.Errorf(
				"filename %q must be %q",
				filename,
				expected,
			))
		}
	}

	if err := errors.Join(problems...); err != nil {
		return nil, err
	}

	return models, nil
}

// LoadJSON проверяет оболочки определений без сборки наследования.
// При ошибках возвращает nil и составную ошибку.
func LoadJSON(documents []json.RawMessage) ([]domain.Schema, error) {
	models := make([]domain.Schema, 0, len(documents))

	var problems []error

	for index, document := range documents {
		model, err := decode(document)
		if err != nil {
			problems = append(
				problems,
				fmt.Errorf("schema %d: %w", index+1, err),
			)

			continue
		}

		models = append(models, model)
	}

	if err := errors.Join(problems...); err != nil {
		return nil, err
	}

	return models, nil
}

func decode(content json.RawMessage) (domain.Schema, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))

	decoder.DisallowUnknownFields()

	var input file

	if err := decoder.Decode(&input); err != nil {
		return domain.Schema{}, fmt.Errorf("decode schema file: %w", err)
	}

	// Первый Decode читает только одно json-значение, поэтому вторым вызовом
	// проверяем, что после него остаются лишь пробелы и конец файла.
	var extra json.RawMessage

	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return domain.Schema{}, fmt.Errorf(
				"decode trailing content: %w",
				err,
			)
		}

		return domain.Schema{}, errors.New("expected one JSON document")
	}

	if err := validate.Struct(input); err != nil {
		return domain.Schema{}, fmt.Errorf("validate schema file: %w", err)
	}

	return domain.Schema{
		ID:        input.ID,
		Parent:    input.Parent,
		OwnSchema: input.Schema,
	}, nil
}
