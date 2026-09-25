package scoring

import "testing"

func TestIsExploration(t *testing.T) {
	tests := []struct {
		line, solution string
		want           bool
	}{
		{"ls", "mv file.txt ../dir3", true},
		{"ls -la; pwd", "mv file.txt ../dir3", true},
		{"cd .. && ls", "mv file.txt ../dir3", true},
		{"cat notas.txt | head", "mv file.txt ../dir3", true},
		{"", "mv file.txt ../dir3", true},
		{"mv file.txt ../dir2", "mv file.txt ../dir3", false},
		{"rm file.txt", "mv file.txt ../dir3", false},
		{"ls > lista.txt", "mv file.txt ../dir3", false},
		{"cat frases.txt", "cat Dewey.txt", false}, // cat está en la solución: es un intento
		{"ls", "cat Dewey.txt", true},
		{"cd dir2", "cd dir1", false},
		{"ls; touch x", "mkdir x", false},
		{"LC_ALL=C ls", "mkdir x", true},
		{"/bin/ls", "mkdir x", true},
		{"grep agua datos.csv", "grep -i agua datos.csv", false},
		{"echo hola", "mkdir x", false},
	}
	for _, tt := range tests {
		if got := IsExploration(tt.line, tt.solution); got != tt.want {
			t.Errorf("IsExploration(%q, %q) = %v, quiero %v", tt.line, tt.solution, got, tt.want)
		}
	}
}
