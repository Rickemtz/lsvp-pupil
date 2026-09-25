package quiz

import (
	"math/rand/v2"
	"slices"
	"testing"

	"lsvp-pupil/internal/content"
)

func TestCheck(t *testing.T) {
	mc := content.Exercise{ID: "mc", Format: content.FormatMultipleChoice, Options: []string{"/etc", "/var"}, Answer: []string{"/etc"}}
	tf := content.Exercise{ID: "tf", Format: content.FormatTrueFalse, Answer: []string{content.False}}
	sa := content.Exercise{ID: "sa", Format: content.FormatShortAnswer, Answer: []string{"Linus Torvalds"}, Accepted: []string{"Torvalds"}}
	ms := content.Exercise{ID: "ms", Format: content.FormatMultiSelect, Options: []string{"bash", "zsh", "kitty"}, Answer: []string{"bash", "zsh"}}

	tests := []struct {
		name  string
		ex    content.Exercise
		given []string
		want  bool
	}{
		{"mc correcta", mc, []string{"/etc"}, true},
		{"mc incorrecta", mc, []string{"/var"}, false},
		{"tf con mayúscula", tf, []string{"Falso"}, true},
		{"tf incorrecta", tf, []string{"Verdadero"}, false},
		{"sa exacta", sa, []string{"Linus Torvalds"}, true},
		{"sa mayúsculas y espacios extremos", sa, []string{"  linus TORVALDS \n"}, true},
		{"sa en accepted", sa, []string{"torvalds"}, true},
		{"sa espacios internos distintos", sa, []string{"linus  torvalds"}, false},
		{"sa vacía", sa, []string{""}, false},
		{"ms mismo conjunto en otro orden", ms, []string{"zsh", "bash"}, true},
		{"ms falta una", ms, []string{"bash"}, false},
		{"ms sobra una", ms, []string{"bash", "zsh", "kitty"}, false},
		{"ms repetida no compensa la que falta", ms, []string{"bash", "bash"}, false},
		{"ms vacía", ms, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Check(tt.ex, tt.given)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Check(%v) = %v, quiero %v", tt.given, got, tt.want)
			}
		})
	}
}

func TestCheckErrors(t *testing.T) {
	mc := content.Exercise{ID: "mc", Format: content.FormatMultipleChoice, Answer: []string{"a"}}
	if _, err := Check(mc, []string{"a", "b"}); err == nil {
		t.Error("dos respuestas en multiple_choice debería dar error")
	}
	if _, err := Check(content.Exercise{Format: "raro"}, []string{"a"}); err == nil {
		t.Error("format desconocido debería dar error")
	}
}

func TestShuffle(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	mc := content.Exercise{Format: content.FormatMultipleChoice, Options: []string{"a", "b", "c", "d"}}
	got := Shuffle(mc, r)
	if !slices.Equal(slices.Sorted(slices.Values(got)), mc.Options) {
		t.Errorf("Shuffle cambió los elementos: %v", got)
	}
	if !slices.Equal(mc.Options, []string{"a", "b", "c", "d"}) {
		t.Error("Shuffle modificó las opciones originales")
	}
	tf := content.Exercise{Format: content.FormatTrueFalse}
	for range 10 {
		if got := Shuffle(tf, r); !slices.Equal(got, []string{content.True, content.False}) {
			t.Fatalf("true_false no debe barajarse: %v", got)
		}
	}
}
