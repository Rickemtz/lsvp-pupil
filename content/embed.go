// Package content embebe los módulos y ejercicios en YAML.
// La carga y validación viven en internal/content.
package content

import "embed"

//go:embed modules.yaml exercises
var Files embed.FS
