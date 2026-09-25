package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo es el estado de un proceso que el setup dejó corriendo en segundo plano.
type ProcessInfo struct {
	Name    string // nombre del programa (comm), p. ej. "sleep"
	Running bool
	Signal  string // si terminó por una señal: "INT", "TERM", "KILL"...; vacío si no se sabe
}

type setupProc struct {
	pid  int // PID como lo ve la shell
	name string
	gen  int // generación de la shell que lo lanzó
}

// Nombres de las señales más comunes, por número (iguales en Linux x86 y arm).
var signalNames = map[int]string{1: "HUP", 2: "INT", 3: "QUIT", 6: "ABRT", 9: "KILL", 10: "USR1", 12: "USR2", 13: "PIPE", 14: "ALRM", 15: "TERM"}

const hiddenTimeout = 3 * time.Second

// recordSetupProcs guarda los trabajos en segundo plano que dejó el setup.
func (b *base) recordSetupProcs() error {
	sh, _ := b.shell()
	raw, err := sh.run(`for __ldj_p in $(jobs -p); do builtin printf '%s %s\n' "$__ldj_p" "$(ps -o comm= -p "$__ldj_p" 2>/dev/null)"; done`,
		hiddenTimeout, DefaultMaxOutput)
	if err != nil {
		return fmt.Errorf("listar los procesos del setup: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw.stdout)), "\n") {
		pid, name, ok := strings.Cut(line, " ")
		n, err := strconv.Atoi(pid)
		if !ok || err != nil {
			continue
		}
		b.setupProcs = append(b.setupProcs, setupProc{pid: n, name: strings.TrimSpace(name), gen: b.gen})
	}
	return nil
}

// Processes dice si siguen vivos los procesos que el setup dejó en segundo plano y, si terminaron,
// por qué señal. Corre un comando oculto en la shell (wait devuelve 128+señal).
func (b *base) Processes() ([]ProcessInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sh, open := b.shell()
	if !open {
		return nil, errors.New("el sandbox ya está cerrado")
	}
	infos := make([]ProcessInfo, len(b.setupProcs))
	var script strings.Builder
	for i, p := range b.setupProcs {
		infos[i].Name = p.name
		if p.gen != b.gen {
			continue // la shell se reinició: su grupo de procesos murió con ella
		}
		// Un zombi (Z) ya terminó aunque ps todavía lo muestre.
		fmt.Fprintf(&script, `__ldj_s=$(ps -o stat= -p %[1]d 2>/dev/null); case "$__ldj_s" in ""|Z*) wait %[1]d 2>/dev/null; builtin echo "%[2]d gone $?";; *) builtin echo "%[2]d alive";; esac`+"\n", p.pid, i)
	}
	if script.Len() == 0 {
		return infos, nil
	}
	raw, err := sh.run(script.String(), hiddenTimeout, DefaultMaxOutput)
	if err != nil {
		return nil, fmt.Errorf("revisar procesos: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw.stdout)), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		i, err := strconv.Atoi(f[0])
		if err != nil || i < 0 || i >= len(infos) {
			continue
		}
		infos[i].Running = f[1] == "alive"
		if f[1] == "gone" && len(f) == 3 {
			if code, err := strconv.Atoi(f[2]); err == nil && code > 128 {
				infos[i].Signal = signalNames[code-128]
			}
		}
	}
	return infos, nil
}

var interactiveCmds = set("less", "more", "man", "htop", "top", "nano", "vim", "vi", "view", "vimtutor", "watch")

// IsInteractive dice si la línea usa un programa de pantalla completa que necesita la terminal real.
func IsInteractive(line string) bool {
	cmds, _ := lex(line)
	for _, c := range cmds {
		words := c.words
		for len(words) > 0 && (reserved[words[0].text] || isAssignment(words[0])) {
			words = words[1:]
		}
		if len(words) > 0 && interactiveCmds[baseName(words[0].text)] {
			return true
		}
	}
	return false
}

func baseName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Interactive prepara una línea con un programa interactivo para correrla con la terminal real
// (la UI se suspende mientras tanto). Pasa por guard como cualquier comando. term es el TERM
// de la terminal real.
func (b *base) Interactive(line, term string) (*exec.Cmd, *Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sh, open := b.shell()
	if !open {
		return nil, nil, errors.New("el sandbox ya está cerrado")
	}
	if blk := b.guard().check(line); blk != nil {
		return nil, blk, nil
	}
	if term == "" {
		term = "xterm-256color"
	}
	return sh.attach(line, b.cwd, term), nil, nil
}

// guard arma el contexto de guard con el estado actual. En el backend dir, kill solo puede
// apuntar a procesos que descienden de la shell del sandbox.
func (b *base) guard() guardContext {
	g := guardContext{home: b.home, cwd: b.cwd, isolated: b.isolated}
	if !b.isolated {
		if sh, _ := b.shell(); sh != nil && sh.cmd.Process != nil {
			shellPid := sh.cmd.Process.Pid
			g.ownsPID = func(pid int) bool { return descendsFrom(pid, shellPid) }
		}
	}
	return g
}

// descendsFrom dice si pid es root o desciende de él, siguiendo el PPID en /proc.
// Sin /proc (macOS) responde falso.
func descendsFrom(pid, root int) bool {
	for i := 0; i < 64 && pid > 1; i++ {
		if pid == root {
			return true
		}
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return false
		}
		// Formato: pid (comm) estado ppid ...; comm puede tener espacios y paréntesis.
		rest := string(data)
		if i := strings.LastIndexByte(rest, ')'); i >= 0 {
			rest = rest[i+1:]
		}
		f := strings.Fields(rest)
		if len(f) < 2 {
			return false
		}
		if pid, err = strconv.Atoi(f[1]); err != nil {
			return false
		}
	}
	return false
}
