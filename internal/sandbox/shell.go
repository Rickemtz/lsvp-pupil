package sandbox

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	errTimeout = errors.New("tiempo agotado")
	errExited  = errors.New("la shell terminó")
)

// shell es un proceso bash persistente. Después de cada comando escribe un marcador en stdout
// (con el código de salida y el directorio actual) y otro en stderr, para saber dónde termina
// la salida de ese comando. Los marcadores llevan un nonce aleatorio para que el usuario no pueda
// imitarlos con echo.
type shell struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	out     chan []byte
	err     chan []byte
	exited  chan struct{} // se cierra cuando el proceso terminó y se leyó toda su salida
	state   *os.ProcessState
	pipes   []io.Closer
	stop    chan struct{} // se cierra al matar la shell: los lectores descartan lo que quede
	stopped sync.Once
	readers sync.WaitGroup
	cleanup func() // limpieza propia del backend al matar la shell; puede ser nil
	attach  func(line, cwd, term string) *exec.Cmd
	outMark []byte
	errMark []byte
	delim   string
}

type rawResult struct {
	stdout, stderr []byte
	exitCode       int
	cwd            string
	truncated      bool
}

func newNonce() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generar nonce: %v", err)) // crypto/rand no falla en Linux ni macOS
	}
	return hex.EncodeToString(b)
}

// startShell arranca cmd (que debe ejecutar bash --noprofile --norc) en su propio grupo de procesos.
func startShell(l launched) (*shell, error) {
	cmd, cleanup := l.cmd, l.cleanup
	nonce := newNonce()
	s := &shell{
		cmd:     cmd,
		cleanup: cleanup,
		attach:  l.attach,
		out:     make(chan []byte, 64),
		err:     make(chan []byte, 64),
		exited:  make(chan struct{}),
		stop:    make(chan struct{}),
		outMark: []byte("__LDJ_END_" + nonce + "__ "),
		errMark: []byte("__LDJ_ERR_" + nonce + "__"),
		delim:   "__LDJ_CMD_" + nonce + "__",
	}
	setProcAttr(cmd)
	var err error
	if s.stdin, err = cmd.StdinPipe(); err != nil {
		return nil, fmt.Errorf("abrir stdin de la shell: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("abrir stdout de la shell: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("abrir stderr de la shell: %w", err)
	}
	if err := cmd.Start(); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, fmt.Errorf("iniciar la shell: %w", err)
	}
	s.pipes = []io.Closer{s.stdin, stdout, stderr}
	s.readers.Add(2)
	go s.readChunks(stdout, s.out)
	go s.readChunks(stderr, s.err)
	go func() {
		// No se usa cmd.Wait: esperaría a que se cierren stdout y stderr, y un «sleep &» que heredó
		// esos pipes los mantiene abiertos después de un exit. Al terminar la shell se mata al resto
		// de su grupo para que los pipes se cierren.
		st, _ := cmd.Process.Wait()
		s.state = st
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		s.readers.Wait()
		for _, p := range s.pipes {
			_ = p.Close()
		}
		close(s.exited)
	}()
	return s, nil
}

// readChunks manda la salida por líneas; una línea muy larga llega en varios trozos.
func (s *shell) readChunks(r io.Reader, ch chan<- []byte) {
	defer s.readers.Done()
	defer close(ch)
	br := bufio.NewReaderSize(r, 32*1024)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			select {
			case ch <- bytes.Clone(line):
			case <-s.stop:
			}
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}

// run ejecuta command y espera a los dos marcadores. La entrada estándar del comando es /dev/null:
// los programas que leen de stdin no se comen el marcador.
func (s *shell) run(command string, timeout time.Duration, limit int) (rawResult, error) {
	// El comando va en un heredoc con delimitador secreto y se ejecuta con eval: así una comilla sin
	// cerrar produce un error de sintaxis en vez de dejar a bash esperando más líneas.
	script := fmt.Sprintf("builtin read -r -d '' __ldj_cmd <<'%[1]s'\n%[2]s\n%[1]s\n"+
		"builtin eval \"$__ldj_cmd\" < /dev/null\n"+
		"builtin printf '\\n%[3]s%%d %%s\\n' \"$?\" \"$PWD\"\n"+
		"builtin printf '\\n%[4]s\\n' >&2\n",
		s.delim, command, s.outMark, s.errMark)
	if _, err := io.WriteString(s.stdin, script); err != nil {
		return rawResult{}, errExited
	}

	var res rawResult
	var outBuf, errBuf bytes.Buffer
	outDone, errDone := false, false
	outStart, errStart := true, true // ¿el siguiente trozo empieza una línea?
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	add := func(buf *bytes.Buffer, chunk []byte) {
		if room := limit - buf.Len(); room < len(chunk) {
			res.truncated = true
			chunk = chunk[:max(0, room)]
		}
		buf.Write(chunk)
	}
	for !outDone || !errDone {
		select {
		case chunk, ok := <-s.out:
			if !ok {
				return s.partial(res, outBuf, errBuf), errExited
			}
			if outStart && bytes.HasPrefix(chunk, s.outMark) {
				code, cwd, _ := strings.Cut(strings.TrimSuffix(string(chunk[len(s.outMark):]), "\n"), " ")
				res.exitCode, _ = strconv.Atoi(code)
				res.cwd = cwd
				outDone = true
				continue
			}
			outStart = bytes.HasSuffix(chunk, []byte("\n"))
			add(&outBuf, chunk)
		case chunk, ok := <-s.err:
			if !ok {
				return s.partial(res, outBuf, errBuf), errExited
			}
			if errStart && bytes.Equal(bytes.TrimSuffix(chunk, []byte("\n")), s.errMark) {
				errDone = true
				continue
			}
			errStart = bytes.HasSuffix(chunk, []byte("\n"))
			add(&errBuf, chunk)
		case <-timer.C:
			return s.partial(res, outBuf, errBuf), errTimeout
		}
	}
	res.stdout = trimMarkerNewline(outBuf.Bytes(), res.truncated)
	res.stderr = trimMarkerNewline(errBuf.Bytes(), res.truncated)
	return res, nil
}

func (s *shell) partial(res rawResult, out, errb bytes.Buffer) rawResult {
	res.stdout, res.stderr = out.Bytes(), errb.Bytes()
	return res
}

// trimMarkerNewline quita el salto de línea que el marcador agrega antes de sí mismo.
func trimMarkerNewline(b []byte, truncated bool) []byte {
	if truncated {
		return b
	}
	return bytes.TrimSuffix(b, []byte("\n"))
}

// kill termina la shell y todo su grupo de procesos (incluidos los que quedaron en segundo plano).
func (s *shell) kill() {
	s.stopped.Do(func() { close(s.stop) })
	if s.cmd.Process != nil {
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
	}
	if s.cleanup != nil {
		s.cleanup()
	}
	select {
	case <-s.exited:
	case <-time.After(2 * time.Second):
	}
}

// exitCode devuelve el código con el que terminó la shell (tras errExited).
func (s *shell) exitCode() int {
	<-s.exited
	if s.state == nil {
		return -1
	}
	return s.state.ExitCode()
}
