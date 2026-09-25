// Package sandbox crea el entorno de cada ejercicio y ejecuta en él los comandos del usuario.
// No conoce la UI: recibe un comando y devuelve un Result.
package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Backend identifica cómo se aísla el sandbox.
type Backend string

const (
	BackendAuto      Backend = "auto"      // el mejor disponible: bwrap, container o dir
	BackendBwrap     Backend = "bwrap"     // bubblewrap: namespaces de Linux, sin red, sistema de solo lectura
	BackendContainer Backend = "container" // docker o podman con la imagen ContainerImage
	BackendDir       Backend = "dir"       // directorio temporal, sin aislamiento real
)

// Valores predeterminados de CLAUDE.md.
const (
	DefaultTimeout   = 5 * time.Second
	DefaultMaxOutput = 64 * 1024
)

// Datos del usuario simulado.
const (
	User     = "alumno"
	Hostname = "lsvp-pupil"
)

// Options configura un sandbox nuevo.
type Options struct {
	Fixtures  fs.FS         // de dónde copiar los archivos de práctica (fixtures.Files)
	Fixture   string        // directorio de Fixtures que se copia al home, p. ej. "taller"; vacío: ninguno
	StartDir  string        // directorio inicial, relativo al home
	Setup     []string      // comandos previos que el usuario no ve; no pasan por guard
	Timeout   time.Duration // límite por comando; 0 = DefaultTimeout
	MaxOutput int           // bytes de salida que se guardan por flujo; 0 = DefaultMaxOutput
}

// Result es lo que produjo un comando.
type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	Cwd       string // directorio actual después del comando, como lo ve la shell
	Duration  time.Duration
	TimedOut  bool   // se agotó el tiempo y se reinició la shell
	Truncated bool   // la salida pasó de MaxOutput
	Restarted bool   // la shell se reinició (por tiempo agotado o porque el comando la terminó): se pierden variables
	Blocked   *Block // guard rechazó el comando y no se ejecutó
}

// Sandbox es un entorno aislado (o no, en el backend dir) con una shell persistente.
type Sandbox interface {
	// Run ejecuta una línea de comandos. El error solo indica fallas del sandbox, no del comando.
	Run(command string) (Result, error)
	// Home es el directorio home como lo ve la shell.
	Home() string
	// Cwd es el directorio actual como lo ve la shell.
	Cwd() string
	// HostPath traduce una ruta relativa al home a una ruta en este equipo, para los validadores.
	HostPath(rel string) string
	Backend() Backend
	// Isolated es falso en el backend dir: la UI debe mostrar un aviso.
	Isolated() bool
	// Processes informa sobre los procesos que el setup dejó en segundo plano.
	Processes() ([]ProcessInfo, error)
	// Interactive prepara una línea con less, man, nano, htop... para correrla con la terminal real.
	Interactive(line, term string) (*exec.Cmd, *Block, error)
	// Close mata todos los procesos y borra el sandbox.
	Close() error
}

// New crea un sandbox con el backend indicado (no acepta BackendAuto: usar Resolve antes).
func New(b Backend, opts Options) (Sandbox, error) {
	switch b {
	case BackendDir:
		return newDir(opts)
	case BackendBwrap:
		return newBwrap(opts)
	case BackendContainer:
		return newContainer(opts)
	}
	return nil, fmt.Errorf("backend de sandbox desconocido %q", b)
}

// launched es lo que un backend prepara para arrancar una shell.
type launched struct {
	cmd *exec.Cmd
	// cleanup limpia lo que matar al proceso no alcanza (p. ej. un contenedor). Puede ser nil.
	cleanup func()
	// attach prepara un programa interactivo que corre junto a esta shell: mismos archivos y,
	// si el backend lo permite, los mismos procesos visibles. cwd es la ruta dentro del sandbox.
	attach func(line, cwd, term string) *exec.Cmd
}

// launcher prepara la shell para el directorio cwd (tal como lo ve la shell).
type launcher func(cwd string) launched

// base tiene la lógica común a todos los backends.
// mu serializa Run; shMu protege sh y closed por poco tiempo, para que Close pueda matar la shell
// sin esperar a que termine el comando en curso.
type base struct {
	mu       sync.Mutex
	shMu     sync.Mutex
	backend  Backend
	isolated bool
	root     string // directorio temporal completo; se borra al cerrar
	hostRoot string // home en este equipo
	home     string // home como lo ve la shell
	cwd      string
	opts     Options
	launch   launcher
	sh       *shell
	gen      int // cambia con cada reinicio de la shell
	closed   bool

	setupProcs []setupProc

	removeOnce sync.Once
	removeErr  error
}

func (b *base) Home() string     { return b.home }
func (b *base) Backend() Backend { return b.backend }
func (b *base) Isolated() bool   { return b.isolated }

func (b *base) Cwd() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cwd
}

func (b *base) HostPath(rel string) string {
	return filepath.Join(b.hostRoot, filepath.FromSlash(path.Clean("/"+rel)))
}

// init copia los fixtures, arranca la shell y corre el setup.
func (b *base) init() error {
	if b.opts.Timeout == 0 {
		b.opts.Timeout = DefaultTimeout
	}
	if b.opts.MaxOutput == 0 {
		b.opts.MaxOutput = DefaultMaxOutput
	}
	if b.opts.Fixture != "" {
		if err := copyFixture(b.opts.Fixtures, b.opts.Fixture, b.hostRoot); err != nil {
			return err
		}
	}
	b.cwd = path.Join(b.home, b.opts.StartDir)
	if !strings.HasPrefix(b.cwd+"/", b.home+"/") {
		return fmt.Errorf("start_dir %q sale del home", b.opts.StartDir)
	}
	if err := os.MkdirAll(b.HostPath(b.opts.StartDir), 0o755); err != nil {
		return fmt.Errorf("crear start_dir: %w", err)
	}
	if err := b.restart(); err != nil {
		return err
	}
	for _, cmd := range b.opts.Setup {
		r, err := b.exec(cmd)
		if err != nil {
			return fmt.Errorf("setup %q: %w", cmd, err)
		}
		if r.ExitCode != 0 {
			return fmt.Errorf("setup %q terminó con código %d: %s", cmd, r.ExitCode, strings.TrimSpace(r.Stderr))
		}
	}
	if len(b.opts.Setup) > 0 {
		return b.recordSetupProcs()
	}
	return nil
}

func (b *base) restart() error {
	b.shMu.Lock()
	defer b.shMu.Unlock()
	if b.closed {
		return errors.New("el sandbox ya está cerrado")
	}
	if b.sh != nil {
		b.sh.kill()
	}
	sh, err := startShell(b.launch(b.cwd))
	if err != nil {
		return err
	}
	b.sh = sh
	b.gen++
	return nil
}

func (b *base) shell() (*shell, bool) {
	b.shMu.Lock()
	defer b.shMu.Unlock()
	return b.sh, !b.closed
}

func (b *base) Run(command string) (Result, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, open := b.shell(); !open {
		return Result{}, errors.New("el sandbox ya está cerrado")
	}
	if strings.TrimSpace(command) == "" {
		return Result{Cwd: b.cwd}, nil
	}
	if blk := b.guard().check(command); blk != nil {
		return Result{Cwd: b.cwd, Blocked: blk}, nil
	}
	return b.exec(command)
}

// exec ejecuta sin pasar por guard. Tras un tiempo agotado o un exit, reinicia la shell en el último
// directorio conocido.
func (b *base) exec(command string) (Result, error) {
	sh, open := b.shell()
	if !open {
		return Result{}, errors.New("el sandbox ya está cerrado")
	}
	start := time.Now()
	raw, err := sh.run(command, b.opts.Timeout, b.opts.MaxOutput)
	res := Result{
		Stdout:    string(raw.stdout),
		Stderr:    string(raw.stderr),
		ExitCode:  raw.exitCode,
		Truncated: raw.truncated,
		Duration:  time.Since(start),
	}
	switch {
	case errors.Is(err, errTimeout):
		res.TimedOut, res.ExitCode = true, 124 // el mismo código que usa timeout(1)
	case errors.Is(err, errExited):
		res.ExitCode = sh.exitCode()
	case err != nil:
		return res, err
	default:
		b.cwd = raw.cwd
		res.Cwd = b.cwd
		return res, nil
	}
	if err := b.restart(); err != nil {
		return res, fmt.Errorf("reiniciar la shell: %w", err)
	}
	res.Restarted = true
	res.Cwd = b.cwd
	return res, nil
}

// Close mata la shell de inmediato (un Run en curso termina con error), espera a que Run suelte el
// sandbox y borra el directorio temporal.
func (b *base) Close() error {
	b.shMu.Lock()
	b.closed = true
	if b.sh != nil {
		b.sh.kill()
	}
	b.shMu.Unlock()
	b.mu.Lock() // espera a que termine un Run en curso
	defer b.mu.Unlock()
	b.removeOnce.Do(func() {
		if b.root != "" {
			b.removeErr = removeSandboxDir(b.root)
		}
	})
	return b.removeErr
}

// copyFixture copia fsys/name a dst/name.
func copyFixture(fsys fs.FS, name, dst string) error {
	if fsys == nil {
		return errors.New("Options.Fixtures es nil")
	}
	return fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("copiar fixture %s: %w", p, err)
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("leer fixture %s: %w", p, err)
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func shellEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"USER=" + User,
		"LOGNAME=" + User,
		"SHELL=/bin/bash",
		"TERM=dumb",
		// Locale fijo: sort, uniq y compañía ordenan igual en cualquier equipo.
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"HISTFILE=/dev/null",
	}
}
