package domain

import (
	"bytes"
	_ "embed"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const dialect = "https://json-schema.org/draft/2020-12/schema"

// definitionSchema метасхема, по которой валидируются тела будущих схем.
// Собирается только через santhosh-tekuri/jsonschema, потому что
// google/jsonschema-go не поддерживает рекурсивный запрет additionalProperties.
//
//go:embed definition.schema.json
var definitionSchema []byte

// compileDefinition компилирует метасхему definition.schema.json.
func compileDefinition() (*jsonschema.Schema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(definitionSchema))
	if err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)

	const location = "urn:schemas:definition"

	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}

	return compiler.Compile(location)
}
