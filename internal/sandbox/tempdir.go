package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const tempPrefix = "lsvp-pupil-"

// newTempRoot crea el directorio temporal de un sandbox y devuelve su ruta real.
func newTempRoot() (string, error) {
	root, err := os.MkdirTemp("", tempPrefix+"*")
	if err != nil {
		return "", fmt.Errorf("crear directorio del sandbox: %w", err)
	}
	// En macOS el directorio temporal pasa por un enlace simbólico; la shell reporta la ruta real.
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return "", fmt.Errorf("resolver directorio del sandbox: %w", err)
	}
	return root, nil
}

// writeMeta escribe en dir un passwd y un group donde el usuario uid:gid se llama alumno.
// bwrap y container los montan sobre /etc para que whoami y ls -l no muestren el usuario real.
func writeMeta(dir string, uid, gid int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear metadatos del sandbox: %w", err)
	}
	passwd := fmt.Sprintf("root:x:0:0:root:/root:/bin/bash\n%s:x:%d:%d:Alumno:%s:/bin/bash\n", User, uid, gid, visibleHome)
	group := fmt.Sprintf("root:x:0:\n%s:x:%d:\n", User, gid)
	for name, data := range map[string]string{"passwd": passwd, "group": group, "hostname": Hostname + "\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			return fmt.Errorf("escribir %s del sandbox: %w", name, err)
		}
	}
	return nil
}

// removeSandboxDir borra el directorio temporal, solo si de verdad es uno nuestro.
func removeSandboxDir(root string) error {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return fmt.Errorf("resolver directorio temporal: %w", err)
	}
	if !filepath.IsAbs(root) || filepath.Dir(root) != tmp || !strings.HasPrefix(filepath.Base(root), tempPrefix) {
		return fmt.Errorf("me niego a borrar %q: no parece un sandbox", root)
	}
	makeWritable(root)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("borrar sandbox %s: %w", root, err)
	}
	return nil
}

// makeWritable devuelve permisos a los directorios (el alumno pudo hacer chmod 000) para poder borrarlos.
// No sigue enlaces simbólicos: chmod sobre un enlace cambiaría el archivo al que apunta, que puede estar fuera.
func makeWritable(root string) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrPermission) {
			return nil
		}
		if d != nil && d.IsDir() && d.Type()&fs.ModeSymlink == 0 {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
}
