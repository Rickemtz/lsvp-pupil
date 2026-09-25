package sandbox

import "testing"

const testHome = "/tmp/lsvp-pupil-x"

func TestGuardAllows(t *testing.T) {
	// Comandos que aparecen en el curso o que un alumno escribiría normalmente.
	allowed := []string{
		"",
		"ls -la",
		"pwd",
		"mkdir -p proyecto/linux/basico",
		"mkdir dir1 dir2 dir3 dir4",
		"touch file.txt documento",
		"cp file.txt file2.txt",
		"cp documento ~/taller/proyecto",
		"cp -r ~/taller/proyecto ~/taller/dir2",
		"mv file.txt ../dir3",
		"mv file.txt ~/taller/dir3",
		"rmdir dir4",
		"rm dir1/file.txt",
		"rm -r proyecto",
		"rm -rf ~/taller/dir1",
		"cat Dewey.txt frases.txt",
		"find ~/taller -name 'eje*.txt'",
		"find / -name '*.txt'", // buscar sin borrar no es destructivo
		"cat red_pipes/poesia_artificial > red_pipes/poesia_redireccionada",
		"ls noexiste 2> error.txt",
		"ls noexiste 2>> errores.txt",
		`echo "Esto es una copia" >> red_pipes/poesia_redireccionada`,
		"grep water datos.csv | wc -l",
		`cut -d "," -f 2,6 datos.csv | head`,
		"sort -t ',' -k 2 datos.csv",
		"tail -n 800 datos.csv | cut -d ',' -f 2 | sort | uniq -c | sort -rn | head -n 1",
		"ls > /dev/null 2>&1",
		"cat /etc/os-release", // leer no es destructivo
		"ls /etc",
		"cd /",
		"cd / && ls",
		"cd .. && ls",
		"cd /tmp && cd ~ && rm -r taller/dir1",
		"ps aux",
		"kill -9 %1",
		"kill -s TERM %1",
		"watch uptime &",
		"echo $HOME",
		"echo 'sudo es peligroso'",
		"echo sudo",
		"echo 'rm -rf /'",
		"# rm -rf /",
		"chmod +x script.sh",
		"./script.sh",
		"bash script.sh",
		"for i in 1 2 3; do echo $i; done",
		"x=5; echo $((x + 1))",
		"if [ -f datos.csv ]; then echo si; fi",
		"diff <(sort a) <(sort b)",
		"echo hola | tee copia.txt",
		"f(){ echo hola; }; f",
		"env LC_ALL=C sort datos.csv",
		"timeout 2 sleep 1",
	}
	g := guardContext{home: testHome, cwd: testHome + "/taller", isolated: false}
	for _, cmd := range allowed {
		if b := g.check(cmd); b != nil {
			t.Errorf("%q bloqueado (regla %d, %q); debería permitirse", cmd, b.Rule, b.Detail)
		}
	}
}

func TestGuardBlocks(t *testing.T) {
	blocked := []struct {
		cmd  string
		rule Rule
	}{
		{"sudo ls", RulePrivilege},
		{"sudo -i", RulePrivilege},
		{"/usr/bin/sudo ls", RulePrivilege},
		{"su", RulePrivilege},
		{"su - root", RulePrivilege},
		{"ls && sudo rm x", RulePrivilege},
		{"echo hola | sudo tee /etc/x", RulePrivilege},
		{"env sudo ls", RulePrivilege},
		{"nohup sudo ls &", RulePrivilege},
		{"FOO=1 sudo ls", RulePrivilege},
		{"doas ls", RulePrivilege},
		{"shutdown now", RulePower},
		{"reboot", RulePower},
		{"poweroff", RulePower},
		{"mkfs.ext4 /dev/sda1", RuleMkfs},
		{"mkfs -t ext4 disco.img", RuleMkfs},
		{"dd if=/dev/zero of=/dev/sda", RuleDeviceWrite},
		{"dd if=x.img of=/dev/nvme0n1 bs=4M", RuleDeviceWrite},
		{":(){ :|:& };:", RuleForkBomb},
		{":(){ :|: & };:", RuleForkBomb},
		{"bomba(){ bomba | bomba & }; bomba", RuleForkBomb},
		{"rm -rf /", RuleOutsidePath},
		{"rm -rf /*", RuleOutsidePath},
		{"rm -rf /etc", RuleOutsidePath},
		{"rm -rf -- /etc", RuleOutsidePath},
		{"rm -rf ../../..", RuleOutsidePath},
		{"rm -rf ../../*", RuleOutsidePath},
		{"rm -rf ~/../..", RuleOutsidePath},
		{"rm -rf ~root", RuleOutsidePath},
		{"cd / && rm -rf usr", RuleOutsidePath},
		{"cd /; rm -rf usr", RuleOutsidePath},
		{"cd ../..; rm -r tmp", RuleOutsidePath},
		{"mv datos.csv /tmp", RuleOutsidePath},
		{"cp -r . /home/otro", RuleOutsidePath},
		{"cp --target-directory=/etc x", RuleOutsidePath},
		{"chmod 777 /etc/passwd", RuleOutsidePath},
		{"chown alumno /usr/bin", RuleOutsidePath},
		{"ln -s / raiz", RuleOutsidePath},
		{"truncate -s 0 /var/log/syslog", RuleOutsidePath},
		{"mkdir /tmp/x", RuleOutsidePath},
		{"mkdir -p ../../fuera", RuleOutsidePath},
		{"touch /etc/nologin", RuleOutsidePath},
		{"echo hola > /etc/motd", RuleOutsidePath},
		{"echo hola >> /home/otro/.bashrc", RuleOutsidePath},
		{"ls &> /tmp/salida", RuleOutsidePath},
		{"ls 2> /tmp/errores", RuleOutsidePath},
		{"echo x | tee /etc/hosts", RuleOutsidePath},
		{"find / -name '*.log' -delete", RuleOutsidePath},
		{`find /home -exec rm {} \;`, RuleOutsidePath},
		{`bash -c "rm -rf /"`, RuleOutsidePath},
		{`sh -c 'sudo ls'`, RulePrivilege},
		{`eval "rm -rf /"`, RuleOutsidePath},
		{"echo $(rm -rf /)", RuleOutsidePath},
		{"echo `sudo ls`", RulePrivilege},
		{`echo "$(rm -rf /etc)"`, RuleOutsidePath},
		{"(cd /; rm -rf etc)", RuleOutsidePath},
		{"if true; then rm -rf /; fi", RuleOutsidePath},
		{"true && rm -rf /", RuleOutsidePath},
		{"r\\m -rf /", RuleOutsidePath},
		{"'rm' -rf /", RuleOutsidePath},
		{"rm -rf \"/\"", RuleOutsidePath},
		{"command rm -rf /", RuleOutsidePath},
		{"timeout 5 rm -rf /", RuleOutsidePath},
		{"rm -rf ~", RuleHomeWipe},
		{"rm -rf ~/", RuleHomeWipe},
		{"rm -rf ..", RuleHomeWipe}, // desde ~/taller
		{"rm -rf $HOME/..", RuleUnverifiable},
		{"rm -rf $DIR", RuleUnverifiable},
		{"echo / | xargs rm -rf", RuleUnverifiable},
		{"cd $X && rm -rf datos", RuleUnverifiable},
		{"cd - && rm -rf datos", RuleUnverifiable},
	}
	g := guardContext{home: testHome, cwd: testHome + "/taller", isolated: false}
	for _, tt := range blocked {
		b := g.check(tt.cmd)
		switch {
		case b == nil:
			t.Errorf("%q permitido; debería bloquearse (regla %d)", tt.cmd, tt.rule)
		case b.Rule != tt.rule:
			t.Errorf("%q bloqueado con regla %d (%q), quiero %d", tt.cmd, b.Rule, b.Detail, tt.rule)
		}
	}
}

// Con aislamiento real (bwrap, contenedor) guard no bloquea lo que no puede comprobar,
// pero sigue enseñando: rutas fuera del sandbox, sudo, etc.
func TestGuardIsolated(t *testing.T) {
	g := guardContext{home: testHome, cwd: testHome + "/taller", isolated: true}
	for _, cmd := range []string{"rm -rf $DIR", "echo a | xargs rm", "rm -rf ~", "cd $X && rm -rf datos"} {
		if b := g.check(cmd); b != nil {
			t.Errorf("aislado: %q bloqueado (regla %d)", cmd, b.Rule)
		}
	}
	for _, cmd := range []string{"sudo ls", "rm -rf /etc", ":(){ :|:& };:"} {
		if g.check(cmd) == nil {
			t.Errorf("aislado: %q permitido", cmd)
		}
	}
}

func TestLex(t *testing.T) {
	cmds, nested := lex(`echo "a b" 'c d' e\ f > out.txt 2>&1; ls $(pwd) | wc -l`)
	if len(cmds) != 3 {
		t.Fatalf("comandos = %d: %+v", len(cmds), cmds)
	}
	want := []string{"echo", "a b", "c d", "e f"}
	for i, w := range want {
		if cmds[0].words[i].text != w {
			t.Errorf("palabra %d = %q, quiero %q", i, cmds[0].words[i].text, w)
		}
	}
	if len(cmds[0].words) != 4 || len(cmds[0].redirects) != 1 || cmds[0].redirects[0].text != "out.txt" {
		t.Errorf("redirecciones = %+v, palabras = %+v", cmds[0].redirects, cmds[0].words)
	}
	if !cmds[1].words[1].expands || len(nested) != 1 || nested[0] != "pwd" {
		t.Errorf("sustitución: %+v %v", cmds[1].words, nested)
	}
}
