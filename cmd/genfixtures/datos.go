package main

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
)

const datosHeader = "id,nombre,tipo1,tipo2,total,hp,ataque,defensa,atq_esp,def_esp,velocidad,generacion,legendario"

const (
	datosRecords      = 800 // así "tail -n 800 datos.csv" deja fuera solo el encabezado, como en el curso
	datosGeneraciones = 6
	legendarioSi      = "Verdadero"
	legendarioNo      = "Falso"
)

// Pesos de cada tipo: Agua es claramente el más frecuente para la pregunta «¿qué tipo hay más?».
var tipos = []struct {
	name   string
	weight int
}{
	{"Agua", 18}, {"Normal", 11}, {"Planta", 9}, {"Bicho", 8}, {"Psíquico", 7}, {"Fuego", 7},
	{"Eléctrico", 6}, {"Roca", 6}, {"Tierra", 5}, {"Veneno", 4}, {"Lucha", 4}, {"Fantasma", 4},
	{"Dragón", 4}, {"Siniestro", 4}, {"Acero", 3}, {"Hielo", 3}, {"Hada", 3}, {"Volador", 3},
}

var (
	silabasInicio = []string{"Bra", "Cha", "Dre", "Flo", "Gru", "Hu", "Ixi", "Ja", "Ko", "Lu", "Mo", "Nu",
		"Pa", "Que", "Ri", "Sa", "Te", "Vo", "Xo", "Za", "Tla", "Ze", "Pi", "Cue"}
	silabasMedio = []string{"ra", "li", "mo", "ta", "chi", "ne", "zu", "pe", "lo", "ca", "vi", "do", "", ""}
	silabasFin   = []string{"lín", "tor", "rón", "ix", "zel", "mon", "sar", "pio", "nta", "co", "ble", "tzin"}
	// Palabras de tipo que aparecen dentro de algunos nombres, en minúsculas o al inicio,
	// para que «grep agua» y «grep -i agua» den resultados distintos.
	sufijosTipo = []string{"agua", "fuego", "planta", "roca", "hielo"}
)

type criatura struct {
	nombre, tipo1, tipo2 string
	stats                [6]int
	generacion           int
	legendario           bool
}

func genDatos(r *rand.Rand) string {
	usados := map[string]bool{}
	cs := make([]criatura, 0, datosRecords)
	for len(cs) < datosRecords {
		n := nombre(r)
		if usados[n] {
			continue
		}
		usados[n] = true
		c := criatura{nombre: n, tipo1: tipo(r), generacion: 1 + r.IntN(datosGeneraciones)}
		if r.IntN(100) < 48 {
			for c.tipo2 == "" || c.tipo2 == c.tipo1 {
				c.tipo2 = tipo(r)
				if r.IntN(100) < 20 {
					c.tipo2 = "Volador"
				}
			}
		}
		c.legendario = r.IntN(100) < 5
		for i := range c.stats {
			s := 20 + r.IntN(111)
			if c.legendario {
				s = min(200, s+40)
			}
			c.stats[i] = s
		}
		cs = append(cs, c)
	}
	// Como en una enciclopedia: los ids avanzan por generación.
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].generacion < cs[j].generacion })

	var b strings.Builder
	b.WriteString(datosHeader + "\n")
	for i, c := range cs {
		total := 0
		for _, s := range c.stats {
			total += s
		}
		leg := legendarioNo
		if c.legendario {
			leg = legendarioSi
		}
		fmt.Fprintf(&b, "%d,%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%d,%s\n", i+1, c.nombre, c.tipo1, c.tipo2, total,
			c.stats[0], c.stats[1], c.stats[2], c.stats[3], c.stats[4], c.stats[5], c.generacion, leg)
	}
	return b.String()
}

func nombre(r *rand.Rand) string {
	switch r.IntN(20) {
	case 0: // tipo al inicio: «Fuegorrón»
		t := sufijosTipo[r.IntN(len(sufijosTipo))]
		return strings.ToUpper(t[:1]) + t[1:] + silabasFin[r.IntN(len(silabasFin))]
	case 1: // tipo al final, en minúsculas: «Tlazufuego»
		return silabasInicio[r.IntN(len(silabasInicio))] + silabasMedio[r.IntN(len(silabasMedio))] +
			sufijosTipo[r.IntN(len(sufijosTipo))]
	}
	return silabasInicio[r.IntN(len(silabasInicio))] + silabasMedio[r.IntN(len(silabasMedio))] +
		silabasFin[r.IntN(len(silabasFin))]
}

func tipo(r *rand.Rand) string {
	total := 0
	for _, t := range tipos {
		total += t.weight
	}
	n := r.IntN(total)
	for _, t := range tipos {
		if n < t.weight {
			return t.name
		}
		n -= t.weight
	}
	panic("inalcanzable")
}
