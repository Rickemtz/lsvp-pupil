package sandbox

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ParseBackend valida el nombre de un backend (flag --sandbox).
func ParseBackend(name string) (Backend, error) {
	switch b := Backend(name); b {
	case BackendAuto, BackendBwrap, BackendContainer, BackendDir:
		return b, nil
	}
	return "", fmt.Errorf("backend de sandbox desconocido %q (opciones: auto, bwrap, container, dir)", name)
}

// Resolve convierte auto en el mejor backend disponible y comprueba que uno pedido explícitamente funcione.
func Resolve(b Backend) (Backend, error) {
	switch b {
	case BackendAuto:
		if bwrapAvailable() == nil {
			return BackendBwrap, nil
		}
		if containerAvailable() == nil {
			return BackendContainer, nil
		}
		return BackendDir, nil
	case BackendBwrap:
		if err := bwrapAvailable(); err != nil {
			return "", fmt.Errorf("el backend bwrap no está disponible: %w", err)
		}
	case BackendContainer:
		if err := containerAvailable(); err != nil {
			return "", fmt.Errorf("el backend container no está disponible: %w", err)
		}
	case BackendDir:
	default:
		return "", fmt.Errorf("backend de sandbox desconocido %q", b)
	}
	return b, nil
}

var (
	bwrapOnce sync.Once
	bwrapErr  error
)

// bwrapAvailable comprueba una sola vez que bwrap exista y pueda crear namespaces
// (algunas distribuciones los restringen para usuarios sin privilegios).
func bwrapAvailable() error {
	bwrapOnce.Do(func() {
		path, err := exec.LookPath("bwrap")
		if err != nil {
			bwrapErr = errors.New("bwrap no está instalado")
			return
		}
		out, err := exec.Command(path, "--unshare-all", "--die-with-parent", "--ro-bind", "/", "/", "true").CombinedOutput()
		if err != nil {
			bwrapErr = fmt.Errorf("bwrap no puede crear el sandbox: %s", strings.TrimSpace(string(out)))
		}
	})
	return bwrapErr
}

var (
	containerOnce    sync.Once
	containerRuntime string
	containerErr     error
)

// containerAvailable busca podman o docker (en ese orden: podman corre sin root por omisión)
// y comprueba que la imagen del sandbox exista.
func containerAvailable() error {
	containerOnce.Do(func() {
		for _, name := range []string{"podman", "docker"} {
			path, err := exec.LookPath(name)
			if err != nil {
				continue
			}
			if err := exec.Command(path, "image", "inspect", ContainerImage).Run(); err != nil {
				containerErr = fmt.Errorf("falta la imagen %s (créala con «make image»)", ContainerImage)
				return
			}
			containerRuntime = path
			return
		}
		containerErr = errors.New("no se encontró podman ni docker")
	})
	return containerErr
}
