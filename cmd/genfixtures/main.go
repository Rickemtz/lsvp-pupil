// Command genfixtures genera los archivos de práctica deterministas (semilla fija) en fixtures/.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
)

// Semilla fija: los mismos archivos en cualquier máquina, para que las respuestas esperadas no cambien.
const seed1, seed2 = 2024, 3

// Directorios que genfixtures administra dentro de out; nada más se borra.
var roots = []string{"taller", "taller2"}

func main() {
	out := flag.String("out", "fixtures", "directorio de salida")
	flag.Parse()
	if err := write(*out, generate()); err != nil {
		fmt.Fprintf(os.Stderr, "genfixtures: %v\n", err)
		os.Exit(1)
	}
}

// generate devuelve el contenido de cada archivo por ruta relativa.
func generate() map[string]string {
	r := rand.New(rand.NewPCG(seed1, seed2))
	red := "taller2/dia3/redireccionYPipes/red_pipes/"
	return map[string]string{
		"taller/archivo.txt":          textArchivo,
		"taller/dia-1/Dewey.txt":      textDewey,
		"taller/dia-1/frases.txt":     textFrases,
		"taller/eje1.md":              textEje1,
		"taller/eje_backup.log":       textEjeBackup,
		"taller/ejeDatos.txt":         textEjeDatos,
		"taller/ejemplo.txt":          textEjemplo,
		"taller/eje_practica.txt":     textEjePractica,
		"taller/ejes.txt":             textEjes,
		"taller/notas_eje1.txt":       textNotasEje1,
		"taller/.oculto":              textOculto,
		red + "datos.csv":             genDatos(r),
		red + "poesia_artificial":     textPoesia,
		red + "poesia_redireccionada": "",
	}
}

func write(out string, files map[string]string) error {
	for _, root := range roots {
		if err := os.RemoveAll(filepath.Join(out, root)); err != nil {
			return fmt.Errorf("limpiar %s: %w", root, err)
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(out, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("crear directorio de %s: %w", p, err)
		}
		if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
			return fmt.Errorf("escribir %s: %w", p, err)
		}
	}
	return nil
}
