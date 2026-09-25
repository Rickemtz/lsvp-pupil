package sandbox

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestProcesses(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		setup := []string{
			"sleep 100000 &",
			"env --default-signal=INT sleep 100001 > /dev/null 2>&1 &",
		}
		sb := newTestSandboxOn(t, b, Options{Setup: setup})
		ps, err := sb.Processes()
		if err != nil {
			t.Fatal(err)
		}
		if len(ps) != 2 || ps[0].Name != "sleep" || !ps[0].Running || !ps[1].Running {
			t.Fatalf("procesos al inicio: %+v", ps)
		}
		if r := run(t, sb, "kill -9 %1"); r.Blocked != nil {
			t.Fatalf("kill %%1 bloqueado: %+v", r.Blocked)
		}
		if r := run(t, sb, "kill -s INT %2"); r.Blocked != nil {
			t.Fatalf("kill -s INT %%2 bloqueado: %+v", r.Blocked)
		}
		ps, err = sb.Processes()
		if err != nil {
			t.Fatal(err)
		}
		if ps[0].Running || ps[0].Signal != "KILL" || ps[1].Running || ps[1].Signal != "INT" {
			t.Errorf("después de kill: %+v", ps)
		}
		// Consultar otra vez da lo mismo (wait recuerda el estado).
		if again, _ := sb.Processes(); again[0].Signal != "KILL" {
			t.Errorf("segunda consulta: %+v", again)
		}
	})
}

func TestProcessesAfterRestart(t *testing.T) {
	sb := newTestSandbox(t, Options{Setup: []string{"sleep 100000 &"}})
	run(t, sb, "exit")
	ps, err := sb.Processes()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Running {
		t.Errorf("tras reiniciar la shell el proceso del setup ya no corre: %+v", ps)
	}
}

func TestInteractive(t *testing.T) {
	forEachBackend(t, func(t *testing.T, b Backend) {
		if b == BackendContainer {
			t.Skip("docker exec -t necesita una terminal real")
		}
		sb := newTestSandboxOn(t, b, Options{Setup: []string{"sleep 100002 &"}})
		run(t, sb, "cd dia-1")
		cmd, blk, err := sb.Interactive("pwd; cat Dewey.txt; ps -e -o args", "xterm")
		if err != nil || blk != nil {
			t.Fatalf("Interactive: %v %+v", err, blk)
		}
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("ejecutar: %v", err)
		}
		s := string(out)
		if !strings.HasPrefix(s, sb.Home()+"/taller/dia-1\nDewey\n") {
			t.Errorf("debe correr en el directorio actual del sandbox:\n%s", s)
		}
		// En bwrap, gracias a nsenter, ve los procesos del sandbox.
		if !strings.Contains(s, "sleep 100002") {
			t.Errorf("no ve el proceso del setup:\n%s", s)
		}
		if _, blk, _ := sb.Interactive("sudo nano /etc/hosts", "xterm"); blk == nil {
			t.Error("Interactive también pasa por guard")
		}
	})
}

func TestIsInteractive(t *testing.T) {
	yes := []string{"less datos.csv", "man ls", "htop", "top", "nano notas.txt", "vim x", "vi x",
		"cat datos.csv | less", "LESS=-R less x", "/usr/bin/less x", "watch uptime"}
	no := []string{"ls", "cat less.txt", "echo man", "grep vim notas.txt", "mkdir top"}
	for _, l := range yes {
		if !IsInteractive(l) {
			t.Errorf("IsInteractive(%q) = false", l)
		}
	}
	for _, l := range no {
		if IsInteractive(l) {
			t.Errorf("IsInteractive(%q) = true", l)
		}
	}
}

func TestGuardKill(t *testing.T) {
	mine := map[int]bool{4242: true}
	g := guardContext{home: testHome, cwd: testHome, ownsPID: func(p int) bool { return mine[p] }}
	allowed := []string{"kill 4242", "kill -9 4242", "kill -s TERM 4242", "kill -KILL 4242", "kill %1", "kill -9 %1",
		"kill $!", "kill -l", "kill -s INT -- 4242", "sleep 5 & kill $!"}
	blocked := []string{"kill 1", "kill 999", "kill -9 -1", "kill -s KILL -1", "kill 0", "kill -9 4242 999", "kill $(pgrep firefox)",
		"kill $PID", "pkill sleep", "killall bash", "sudo kill 4242"}
	for _, c := range allowed {
		if b := g.check(c); b != nil {
			t.Errorf("%q bloqueado: %+v", c, b)
		}
	}
	for _, c := range blocked {
		if g.check(c) == nil {
			t.Errorf("%q permitido", c)
		}
	}
	iso := guardContext{home: testHome, cwd: testHome, isolated: true}
	for _, c := range []string{"kill 1", "pkill sleep", "kill -9 -1"} {
		if b := iso.check(c); b != nil {
			t.Errorf("aislado: %q bloqueado", c)
		}
	}
}

func TestGuardKillRealPIDs(t *testing.T) {
	sb := newTestSandbox(t, Options{})
	r := run(t, sb, "sleep 100003 & echo $!")
	pid := strings.TrimSpace(r.Stdout)
	if r := run(t, sb, "kill "+pid); r.Blocked != nil {
		t.Errorf("kill de un hijo de la shell bloqueado: %+v", r.Blocked)
	}
	if r := run(t, sb, "kill "+strconv.Itoa(os.Getpid())); r.Blocked == nil || r.Blocked.Rule != RuleForeignProcess {
		t.Errorf("kill de un proceso ajeno (este test) debería bloquearse: %+v", r)
	}
}
