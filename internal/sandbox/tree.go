package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxTreeLines evita que un árbol enorme llene la pantalla.
const maxTreeLines = 200

// Tree dibuja el árbol de archivos del home del sandbox, al estilo del comando tree.
// Incluye archivos ocultos y no sigue enlaces simbólicos.
func Tree(sb Sandbox) string {
	var lines []string
	var walk func(dir, prefix string)
	walk = func(dir, prefix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			lines = append(lines, prefix+"└── (sin permiso de lectura)")
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for i, e := range entries {
			if len(lines) >= maxTreeLines {
				return
			}
			branch, next := "├── ", "│   "
			if i == len(entries)-1 {
				branch, next = "└── ", "    "
			}
			name := e.Name()
			full := filepath.Join(dir, name)
			switch {
			case e.Type()&os.ModeSymlink != 0:
				target, _ := os.Readlink(full)
				lines = append(lines, prefix+branch+name+" -> "+target)
			case e.IsDir():
				lines = append(lines, prefix+branch+name+"/")
				walk(full, prefix+next)
			default:
				lines = append(lines, prefix+branch+name)
			}
		}
	}
	walk(sb.HostPath(""), "")
	if len(lines) >= maxTreeLines {
		lines = append(lines[:maxTreeLines], fmt.Sprintf("… (más de %d entradas)", maxTreeLines))
	}
	return strings.Join(append([]string{"~"}, lines...), "\n")
}
