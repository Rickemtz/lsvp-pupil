package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	files "lsvp-pupil/content"
	"lsvp-pupil/fixtures"
	"lsvp-pupil/internal/content"
	"lsvp-pupil/internal/sandbox"
	"lsvp-pupil/internal/storage"
	"lsvp-pupil/internal/ui"
)

// version se fija al compilar: go build -ldflags "-X main.version=v1.0.0".
var version = "dev"

func main() {
	backend := flag.String("sandbox", string(sandbox.BackendAuto), "backend del sandbox para los ejercicios prácticos: auto, bwrap, container o dir")
	unlockAll := flag.Bool("unlock-all", false, "abre todos los módulos sin completar los anteriores")
	showVersion := flag.Bool("version", false, "muestra la versión y termina")
	flag.Parse()
	if *showVersion {
		fmt.Println("lsvp-pupil", version)
		return
	}
	b, err := sandbox.ParseBackend(*backend)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if b, err = sandbox.Resolve(b); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cat, err := content.Load(files.Files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cargar contenido: %v\n", err)
		os.Exit(1)
	}
	var store *storage.Store
	if dir, err := storage.DefaultDir(); err == nil {
		store = storage.Open(dir)
	} else {
		fmt.Fprintf(os.Stderr, "%v; el progreso no se guardará\n", err)
	}
	app := ui.NewApp(ui.Config{Catalog: cat, Backend: b, UnlockAll: *unlockAll, Fixtures: fixtures.Files, Store: store, Version: version})
	p := tea.NewProgram(app, tea.WithAltScreen())
	// Al cerrar la ventana de la terminal llega SIGHUP. Bubble Tea no lo atiende y el programa moriría
	// sin borrar el sandbox; así sale por el camino normal y se limpia abajo.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		<-hup
		p.Quit()
	}()
	final, err := p.Run()
	if m, ok := final.(ui.App); ok {
		m.Close() // por si el programa terminó por una señal y no por F10
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejecutar la interfaz: %v\n", err)
		os.Exit(1)
	}
}
