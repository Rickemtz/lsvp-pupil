// Package fixtures embebe los archivos de práctica que genera cmd/genfixtures (make fixtures).
package fixtures

import "embed"

// "all:" incluye los archivos ocultos, como taller/.oculto.
//
//go:embed all:taller all:taller2
var Files embed.FS
