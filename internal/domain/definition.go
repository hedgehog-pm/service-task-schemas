package domain

import _ "embed"

const dialect = "https://json-schema.org/draft/2020-12/schema"

// definitionSchema описывает допустимое тело схемы.
// additionalProperties: false запрещён и во вложенных определениях.
//
//go:embed definition.schema.json
var definitionSchema []byte
