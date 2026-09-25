package sandbox

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Rule identifica por qué guard rechazó un comando. La UI traduce cada regla a un mensaje educativo.
type Rule int

const (
	RulePrivilege      Rule = iota + 1 // sudo, su...
	RulePower                          // shutdown, reboot...
	RuleMkfs                           // mkfs*
	RuleDeviceWrite                    // dd of=/dev/...
	RuleForkBomb                       // :(){ :|:& };:
	RuleOutsidePath                    // ruta fuera del sandbox en un comando destructivo o en una redirección
	RuleHomeWipe                       // rm del home completo (backend dir)
	RuleUnverifiable                   // backend dir: argumento de un comando destructivo que no se puede comprobar
	RuleForeignProcess                 // backend dir: kill/pkill/killall que podría alcanzar procesos fuera del sandbox
)

// Block es el resultado de un comando rechazado.
type Block struct {
	Rule   Rule
	Detail string // el comando o la ruta que provocó el bloqueo
}

// guardContext es lo que guard necesita saber del sandbox. Las rutas son las que ve la shell.
type guardContext struct {
	home     string
	cwd      string
	isolated bool // falso en el backend dir: ahí guard es la única protección y se vuelve más estricto
	// ownsPID dice si un PID pertenece al sandbox (solo en el backend dir; nil si no se puede saber).
	ownsPID func(pid int) bool
}

var (
	privilegeCmds = set("sudo", "su", "doas", "pkexec", "run0")
	powerCmds     = set("shutdown", "reboot", "poweroff", "halt", "init", "telinit")
	// Comandos que crean, modifican o borran lo que reciben como argumento.
	destructiveCmds = set("rm", "rmdir", "mv", "cp", "chmod", "chown", "chgrp", "ln", "truncate",
		"shred", "tee", "unlink", "install", "dd", "mkdir", "touch")
	shells = set("bash", "sh", "zsh", "dash", "ksh", "fish")
	// Comandos que ejecutan a otro comando que va después de sus opciones.
	wrappers = set("env", "nohup", "time", "command", "builtin", "exec", "nice", "timeout", "stdbuf", "xargs", "setsid")
	// Palabras reservadas que pueden ir antes de un comando.
	reserved = set("if", "then", "else", "elif", "fi", "do", "done", "while", "until", "!", "{", "}", "esac")
	// Destinos de redirección que no escriben en disco.
	safeDevices = set("/dev/null", "/dev/stdout", "/dev/stderr", "/dev/tty")

	funcDefRe = regexp.MustCompile(`([\w:.-]+)\s*\(\s*\)\s*\{([^}]*)\}`)
)

// isForkBomb detecta una función que se llama a sí misma dos veces unida por un pipe,
// la forma clásica :(){ :|:& };:
func isForkBomb(line string) bool {
	for _, m := range funcDefRe.FindAllStringSubmatch(line, -1) {
		name, body := regexp.QuoteMeta(m[1]), m[2]
		if regexp.MustCompile(`(^|[\s;&|])` + name + `\s*\|\s*` + name + `($|[\s;&|])`).MatchString(body) {
			return true
		}
	}
	return false
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// check revisa una línea de comandos antes de ejecutarla. Es una defensa de mejor esfuerzo:
// no interpreta bash por completo (un script o una variable pueden esconder un rm), por eso el
// aislamiento real lo dan los backends bwrap y container.
func (g guardContext) check(line string) *Block {
	if isForkBomb(line) {
		return &Block{Rule: RuleForkBomb, Detail: strings.TrimSpace(line)}
	}
	cmds, nested := lex(line)
	cwd, cwdKnown := g.cwd, true
	for _, c := range cmds {
		if b := g.checkSimple(c, &cwd, &cwdKnown); b != nil {
			return b
		}
	}
	for _, n := range nested {
		if b := g.check(n); b != nil {
			return b
		}
	}
	return nil
}

func (g guardContext) checkSimple(c simpleCmd, cwd *string, cwdKnown *bool) *Block {
	for _, r := range c.redirects {
		if safeDevices[r.text] || strings.HasPrefix(r.text, "/dev/fd/") {
			continue
		}
		if b := g.checkPath(r, *cwd, *cwdKnown, ">"); b != nil {
			return b
		}
	}

	words := c.words
	for len(words) > 0 && (reserved[words[0].text] || isAssignment(words[0])) {
		words = words[1:]
	}
	fromStdin := false // xargs: los argumentos llegan por la entrada estándar
	for len(words) > 0 {
		name := path.Base(words[0].text)
		switch {
		case privilegeCmds[name]:
			return &Block{Rule: RulePrivilege, Detail: name}
		case powerCmds[name]:
			return &Block{Rule: RulePower, Detail: name}
		case strings.HasPrefix(name, "mkfs"):
			return &Block{Rule: RuleMkfs, Detail: name}
		case shells[name]:
			for i, w := range words[1:] {
				if w.text == "-c" && i+2 < len(words) {
					return g.check(words[i+2].text)
				}
			}
			return nil
		case name == "eval":
			return g.check(joinWords(words[1:]))
		case name == "cd":
			*cwd, *cwdKnown = g.resolveCd(words[1:], *cwd, *cwdKnown)
			return nil
		case wrappers[name]:
			if name == "xargs" {
				fromStdin = true
			}
			words = skipWrapperArgs(name, words[1:])
			continue
		case !g.isolated && (name == "pkill" || name == "killall"):
			return &Block{Rule: RuleForeignProcess, Detail: name}
		case !g.isolated && name == "kill":
			return g.checkKill(words[1:])
		case destructiveCmds[name] || name == "find":
			return g.checkDestructive(name, words[1:], *cwd, *cwdKnown, fromStdin)
		}
		return nil
	}
	return nil
}

func (g guardContext) checkDestructive(name string, args []word, cwd string, cwdKnown, fromStdin bool) *Block {
	if fromStdin && !g.isolated && name != "find" {
		return &Block{Rule: RuleUnverifiable, Detail: "xargs " + name}
	}
	if name == "find" {
		return g.checkFind(args, cwd, cwdKnown)
	}
	options := true
	for _, a := range args {
		switch {
		case options && a.text == "--":
			options = false
			continue
		case options && strings.HasPrefix(a.text, "-"):
			// --target-directory=/etc: revisar el valor.
			if i := strings.IndexByte(a.text, '='); i > 0 {
				v := a
				v.text = a.text[i+1:]
				if b := g.checkPath(v, cwd, cwdKnown, name); b != nil {
					return b
				}
			}
			continue
		}
		p := a
		if name == "dd" {
			k, v, ok := strings.Cut(a.text, "=")
			if !ok || k != "of" { // if= solo lee
				continue
			}
			if k == "of" && strings.HasPrefix(v, "/dev/") && !safeDevices[v] {
				return &Block{Rule: RuleDeviceWrite, Detail: a.text}
			}
			p.text = v
		}
		if b := g.checkPath(p, cwd, cwdKnown, name); b != nil {
			return b
		}
		if name == "rm" && !g.isolated && !a.expands && cwdKnown && g.resolve(a.text, cwd) == g.home {
			return &Block{Rule: RuleHomeWipe, Detail: a.text}
		}
	}
	return nil
}

// checkKill (backend dir) solo permite señales a procesos del sandbox: %N, $! o PIDs que
// descienden de su shell. kill -1 o PIDs negativos apuntarían a todos los procesos del usuario.
func (g guardContext) checkKill(args []word) *Block {
	signalSeen := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		t := a.text
		switch {
		case t == "-l" || t == "-L" || strings.HasPrefix(t, "--list") || strings.HasPrefix(t, "--table"):
			return nil // solo lista señales
		case !signalSeen && (t == "-s" || t == "-n" || t == "--signal"):
			signalSeen = true
			i++
			continue
		case !signalSeen && strings.HasPrefix(t, "-") && t != "-" && t != "--":
			signalSeen = true // -9, -KILL, -SIGTERM
			continue
		case t == "--":
			continue
		case strings.HasPrefix(t, "%") || t == "$!":
			continue // un trabajo de esta misma shell
		case a.expands:
			return &Block{Rule: RuleForeignProcess, Detail: "kill " + t}
		}
		pid, err := strconv.Atoi(t)
		if err != nil {
			continue // kill fallará por su cuenta
		}
		if pid <= 0 || g.ownsPID == nil || !g.ownsPID(pid) {
			return &Block{Rule: RuleForeignProcess, Detail: "kill " + t}
		}
	}
	return nil
}

// checkFind solo se preocupa si find borra o ejecuta algo; entonces revisa las rutas donde busca.
func (g guardContext) checkFind(args []word, cwd string, cwdKnown bool) *Block {
	acts := false
	for _, a := range args {
		switch a.text {
		case "-delete", "-exec", "-execdir", "-ok", "-okdir":
			acts = true
		}
	}
	if !acts {
		return nil
	}
	for _, a := range args {
		if strings.HasPrefix(a.text, "-") || a.text == "(" || a.text == "!" {
			break // empiezan las expresiones
		}
		if b := g.checkPath(a, cwd, cwdKnown, "find"); b != nil {
			return b
		}
	}
	return nil
}

// checkPath bloquea rutas fuera del home del sandbox.
func (g guardContext) checkPath(w word, cwd string, cwdKnown bool, cmd string) *Block {
	if w.expands || (!cwdKnown && !path.IsAbs(w.text) && !strings.HasPrefix(w.text, "~")) {
		if g.isolated {
			return nil
		}
		return &Block{Rule: RuleUnverifiable, Detail: cmd + " " + w.text}
	}
	p := g.resolve(w.text, cwd)
	if p != g.home && !strings.HasPrefix(p, g.home+"/") {
		return &Block{Rule: RuleOutsidePath, Detail: w.text}
	}
	return nil
}

// resolve convierte una ruta (con ~ o relativa a cwd) en absoluta y limpia. Los comodines quedan tal cual:
// "../*" desde el home se resuelve a "<padre>/*", que queda fuera.
func (g guardContext) resolve(p, cwd string) string {
	switch {
	case p == "~":
		return g.home
	case strings.HasPrefix(p, "~/"):
		return path.Clean(g.home + p[1:])
	case strings.HasPrefix(p, "~"):
		return "/" // ~usuario: el home de otra persona
	case path.IsAbs(p):
		return path.Clean(p)
	}
	return path.Join(cwd, p)
}

func (g guardContext) resolveCd(args []word, cwd string, known bool) (string, bool) {
	var dir *word
	for i := range args {
		if !strings.HasPrefix(args[i].text, "-") || args[i].text == "-" {
			dir = &args[i]
			break
		}
	}
	switch {
	case dir == nil:
		return g.home, true
	case dir.expands || dir.text == "-":
		return cwd, false
	case !known && !path.IsAbs(dir.text) && !strings.HasPrefix(dir.text, "~"):
		return cwd, false
	}
	return g.resolve(dir.text, cwd), true
}

func skipWrapperArgs(name string, args []word) []word {
	i := 0
	for i < len(args) {
		t := args[i].text
		switch {
		case strings.HasPrefix(t, "-"):
			i++
			// Opciones que llevan valor aparte.
			if (name == "nice" && t == "-n") || (name == "xargs" && (t == "-I" || t == "-n" || t == "-d" || t == "-P")) ||
				(name == "timeout" && (t == "-s" || t == "-k")) {
				i++
			}
		case name == "env" && strings.Contains(t, "="):
			i++
		case name == "timeout":
			return args[i+1:] // la duración
		default:
			return args[i:]
		}
	}
	return nil
}

func isAssignment(w word) bool {
	name, _, ok := strings.Cut(w.text, "=")
	if !ok || name == "" || w.quotedName {
		return false
	}
	for i, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func joinWords(ws []word) string {
	s := make([]string, len(ws))
	for i, w := range ws {
		s[i] = w.text
	}
	return strings.Join(s, " ")
}

// word es una palabra de la línea ya sin comillas.
type word struct {
	text       string
	expands    bool // tiene $var, $(...), `...` o ${...}: su valor real no se conoce antes de ejecutar
	quotedName bool // empezó con comillas: "A=b" no es una asignación
}

type simpleCmd struct {
	words     []word
	redirects []word // destinos de >, >>, &>, 2>...
}

// lex parte una línea de bash en comandos simples. Devuelve además el texto de las sustituciones
// $(...) y `...` para revisarlas por separado.
func lex(line string) (cmds []simpleCmd, nested []string) {
	rs := []rune(line)
	var (
		cur      simpleCmd
		buf      strings.Builder
		inWord   bool
		w        word
		redirect bool // la siguiente palabra es destino de una redirección de salida
		skipNext bool // la siguiente palabra es destino de < o <<, o un descriptor de >&
	)
	endWord := func() {
		if !inWord {
			return
		}
		w.text = buf.String()
		switch {
		case skipNext:
			skipNext = false
		case redirect:
			cur.redirects = append(cur.redirects, w)
			redirect = false
		default:
			cur.words = append(cur.words, w)
		}
		buf.Reset()
		inWord, w = false, word{}
	}
	endCmd := func() {
		endWord()
		if len(cur.words) > 0 || len(cur.redirects) > 0 {
			cmds = append(cmds, cur)
		}
		cur = simpleCmd{}
	}
	start := func(quoted bool) {
		if !inWord {
			inWord = true
			w.quotedName = quoted
		}
	}
	// dollar procesa lo que sigue a un $ en la posición i y devuelve la nueva posición.
	dollar := func(i int) int {
		w.expands = true
		switch {
		case i+1 < len(rs) && rs[i+1] == '(':
			inner, end := balanced(rs, i+1, '(', ')')
			if !strings.HasPrefix(inner, "(") { // $(( )) es aritmética
				nested = append(nested, inner)
			}
			buf.WriteString("$(" + inner + ")")
			return end
		case i+1 < len(rs) && rs[i+1] == '{':
			inner, end := balanced(rs, i+1, '{', '}')
			buf.WriteString("${" + inner + "}")
			return end
		}
		buf.WriteRune('$')
		return i
	}

	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\\':
			if i+1 < len(rs) {
				i++
				if rs[i] != '\n' {
					start(false)
					buf.WriteRune(rs[i])
				}
			}
		case c == '\'':
			start(true)
			for i++; i < len(rs) && rs[i] != '\''; i++ {
				buf.WriteRune(rs[i])
			}
		case c == '"':
			start(true)
			for i++; i < len(rs) && rs[i] != '"'; i++ {
				switch {
				case rs[i] == '\\' && i+1 < len(rs):
					i++
					buf.WriteRune(rs[i])
				case rs[i] == '$':
					i = dollar(i)
				case rs[i] == '`':
					inner, end := until(rs, i+1, '`')
					nested = append(nested, inner)
					w.expands = true
					i = end
				default:
					buf.WriteRune(rs[i])
				}
			}
		case c == '$':
			start(false)
			i = dollar(i)
		case c == '`':
			start(false)
			inner, end := until(rs, i+1, '`')
			nested = append(nested, inner)
			w.expands = true
			i = end
		case c == '#' && !inWord:
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			endCmd()
		case c == ' ' || c == '\t':
			endWord()
		case c == '\n' || c == ';' || c == '|' || c == '(' || c == ')':
			endCmd()
		case c == '&':
			if i+1 < len(rs) && rs[i+1] == '>' { // &> y &>>
				endWord()
				i++
				if i+1 < len(rs) && rs[i+1] == '>' {
					i++
				}
				redirect = true
				continue
			}
			endCmd()
		case c == '>' || c == '<':
			// Un número pegado antes (2>) es el descriptor, no una palabra.
			if inWord && !w.expands && !w.quotedName && isDigits(buf.String()) {
				buf.Reset()
				inWord, w = false, word{}
			}
			endWord()
			if c == '<' {
				if i+1 < len(rs) && rs[i+1] == '(' { // <(...) sustitución de proceso
					inner, end := balanced(rs, i+1, '(', ')')
					nested = append(nested, inner)
					i = end
					continue
				}
				for i+1 < len(rs) && (rs[i+1] == '<' || rs[i+1] == '-') {
					i++
				}
				skipNext = true
				continue
			}
			for i+1 < len(rs) && (rs[i+1] == '>' || rs[i+1] == '|') {
				i++
			}
			if i+1 < len(rs) && rs[i+1] == '&' { // 2>&1: duplica un descriptor
				i++
				skipNext = true
				continue
			}
			if i+1 < len(rs) && rs[i+1] == '(' { // >(...) sustitución de proceso
				inner, end := balanced(rs, i+1, '(', ')')
				nested = append(nested, inner)
				i = end
				continue
			}
			redirect = true
		default:
			start(false)
			buf.WriteRune(c)
		}
	}
	endCmd()
	return cmds, nested
}

// balanced devuelve el texto entre rs[open] y su cierre, y la posición del cierre.
func balanced(rs []rune, open int, o, c rune) (string, int) {
	depth := 0
	for i := open; i < len(rs); i++ {
		switch rs[i] {
		case o:
			depth++
		case c:
			depth--
			if depth == 0 {
				return string(rs[open+1 : i]), i
			}
		}
	}
	return string(rs[open+1:]), len(rs) - 1
}

func until(rs []rune, from int, end rune) (string, int) {
	for i := from; i < len(rs); i++ {
		if rs[i] == end {
			return string(rs[from:i]), i
		}
	}
	return string(rs[from:]), len(rs) - 1
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
