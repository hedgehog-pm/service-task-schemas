// Package schemas загружает seed-файлы json-схем.
package schemas

import "embed"

// FS содержит файлы из этого каталога.
//
//go:embed *
var FS embed.FS
