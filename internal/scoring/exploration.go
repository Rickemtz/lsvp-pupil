package scoring

import (
	"path"
	"strings"
)

// explorationCmds solo miran: sirven para orientarse y no modifican nada.
var explorationCmds = map[string]bool{
	"ls": true, "pwd": true, "cd": true, "cat": true, "less": true, "more": true, "head": true,
	"tail": true, "clear": true, "man": true, "tree": true, "file": true, "stat": true,
	"whoami": true, "history": true, "help": true, "type": true, "which": true,
	"ps": true, "pgrep": true, "jobs": true, "top": true, "htop": true, "uptime": true, "id": true,
}

// IsExploration dice si una línea es solo exploración y por eso no cuenta como intento fallido.
// Lo es si todos sus comandos son de exploración, no redirige a archivos y ninguno de sus comandos
// aparece en la solución: en un ejercicio que se resuelve con cat, un cat equivocado sí es un intento.
func IsExploration(line, solution string) bool {
	if strings.Contains(line, ">") {
		return false
	}
	inSolution := map[string]bool{}
	for _, c := range commandNames(solution) {
		inSolution[c] = true
	}
	names := commandNames(line)
	if len(names) == 0 {
		return true
	}
	for _, c := range names {
		if !explorationCmds[c] || inSolution[c] {
			return false
		}
	}
	return true
}

// commandNames devuelve el nombre de cada comando de la línea. Es una aproximación: no respeta comillas.
func commandNames(line string) []string {
	var names []string
	for _, seg := range strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(";|&\n()", r) }) {
		for _, w := range strings.Fields(seg) {
			if strings.Contains(w, "=") && !strings.HasPrefix(w, "-") { // VAR=valor antes del comando
				continue
			}
			names = append(names, path.Base(w))
			break
		}
	}
	return names
}
