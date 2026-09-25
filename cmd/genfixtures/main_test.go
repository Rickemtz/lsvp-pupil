package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeterministic(t *testing.T) {
	a, b := generate(), generate()
	for p, v := range a {
		if b[p] != v {
			t.Fatalf("%s cambia entre ejecuciones", p)
		}
	}
}

// Si falla, corre «make fixtures» y agrega los cambios.
func TestFixturesUpToDate(t *testing.T) {
	for p, want := range generate() {
		got, err := os.ReadFile(filepath.Join("..", "..", "fixtures", filepath.FromSlash(p)))
		if err != nil {
			t.Errorf("%s: %v (corre make fixtures)", p, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s está desactualizado (corre make fixtures)", p)
		}
	}
}

func TestDatosShape(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(generate()["taller2/dia3/redireccionYPipes/red_pipes/datos.csv"], "\n"), "\n")
	if lines[0] != datosHeader {
		t.Fatalf("encabezado = %q", lines[0])
	}
	if len(lines) != datosRecords+1 {
		t.Fatalf("%d líneas, quiero %d", len(lines), datosRecords+1)
	}
	cols := len(strings.Split(datosHeader, ","))
	names := map[string]bool{}
	counts := map[string]int{}
	for i, l := range lines[1:] {
		f := strings.Split(l, ",")
		if len(f) != cols {
			t.Fatalf("línea %d tiene %d columnas: %q", i+2, len(f), l)
		}
		if names[f[1]] {
			t.Errorf("nombre repetido %q", f[1])
		}
		names[f[1]] = true
		counts[f[2]]++
	}
	top, second := 0, 0
	for _, n := range counts {
		if n > top {
			top, second = n, top
		} else if n > second {
			second = n
		}
	}
	if counts["Agua"] != top || top-second < 20 {
		t.Errorf("Agua debe ser claramente el tipo más frecuente: %v", counts)
	}
}

func TestWriteOnlyTouchesItsRoots(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "embed.go")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := write(dir, map[string]string{"taller/a.txt": "a"}); err != nil {
		t.Fatal(err)
	}
	if err := write(dir, map[string]string{"taller/b.txt": "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("write borró un archivo fuera de taller/ y taller2/")
	}
	if _, err := os.Stat(filepath.Join(dir, "taller", "a.txt")); !os.IsNotExist(err) {
		t.Error("write debe limpiar taller/ antes de escribir")
	}
}
