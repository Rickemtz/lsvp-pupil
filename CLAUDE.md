# CLAUDE.md — Linuxpupil

Guía de trabajo para Claude Code en este repositorio. Léela completa antes de cada tarea.

## Qué es este proyecto

**Linuxpupil** es una aplicación de terminal (TUI) para aprender y evaluar conocimientos de la terminal GNU/Linux. Tiene:

1. **Lecciones progresivas** por módulos.
2. **Contrarreloj**: cada ejercicio tiene límite de tiempo.
3. **Sistema de puntos** con combos y un **puntaje final con rango** (S, A, B, C, D) guardado en un ranking local.

El contenido y los módulos se basan en el curso **"Terminal GNU/Linux Nivel cero"** del Laboratorio de Supercómputo y Visualización en Paralelo (LSVP) de la UAM Iztapalapa:

- Sitio: https://lsvp-uami.github.io/mdbook-curso-linux-0/
- Versión completa en una página: https://lsvp-uami.github.io/mdbook-curso-linux-0/print.html

Hay dos grandes tipos de ejercicio:

- **Teoría** (quiz): opción múltiple, verdadero/falso, respuesta corta. Para conceptos como kernel, historia, jerarquía de directorios, señales o SSH.
- **Práctica**: el usuario escribe **comandos reales de bash** en un sandbox aislado con archivos de práctica. La app valida el estado del sistema de archivos, la salida o el código de salida.

## Uso del material fuente

- El curso es la **referencia de temario**, no un texto para copiar. Las preguntas, explicaciones y pistas se escriben con palabras propias.
- El repositorio del curso (`LSVP-UAMI/mdbook-curso-linux-0`) tiene licencia Apache-2.0 (revisado el 2026-09-23). Aun así, no copiar texto: parafrasear y crear archivos propios. Si algún día se reutiliza un archivo tal cual, conservar el aviso de licencia y atribución.
- Cada ejercicio indica en el campo `source` la sección del curso en la que se basa (URL con ancla).
- La pantalla "Acerca de" y el README dan crédito al LSVP-UAMI y enlazan al curso.
- Los archivos de práctica (`taller/`, `datos.csv`, etc.) se **generan** en este repo imitando la estructura de los del curso, no se descargan.

## Stack

- **Lenguaje:** Go (≥ 1.24.2: lo exigen las dependencias de Charm; `go.mod` dice `go 1.24.2`). Módulo Go: `lsvp-pupil`.
- **TUI:** Bubble Tea + Lip Gloss + Bubbles (textinput, textarea, list, viewport, timer).
- **Contenido:** YAML embebido con `go:embed` (`gopkg.in/yaml.v3`).
- **Ejecución de comandos:** `os/exec` con un proceso bash persistente por ejercicio (ver "Sandbox").
- **Persistencia:** JSON en `~/.config/lsvp-pupil/`.
- **Tests:** `testing` estándar con tablas de casos.

No agregar dependencias nuevas sin preguntar primero.

### Plataformas

- **Linux**: plataforma principal y objetivo real del curso.
- **macOS**: funciona, pero las herramientas BSD (`sed`, `sort`, `find`, `ps`) difieren de GNU. Recomendar modo contenedor.
- **Windows**: solo vía WSL.

## Comandos

```bash
make run        # go run ./cmd/lsvp-pupil  (flags: --sandbox auto|bwrap|container|dir, --unlock-all, --version)
make build      # binario en ./bin/lsvp-pupil
make test       # go test ./...
make lint       # go vet ./... && gofmt -l .
make validate   # ejecuta la solución de cada ejercicio práctico en un sandbox y comprueba que valida
make fixtures   # regenera los archivos de práctica (datos.csv, taller/, ...)
make image      # construye la imagen del backend container (podman o docker)
make release    # corre test, lint y validate; binarios linux/darwin amd64/arm64 en dist/ con SHA256SUMS
make clean      # borra bin/ y dist/
```

Antes de dar una tarea por terminada: `make test`, `make lint` y `make validate` deben pasar.

La versión se inyecta al compilar (`-X main.version=...`) desde `git describe --tags --always --dirty`; fuera de un repositorio es `dev`. Para publicar: etiquetar (`git tag vX.Y.Z`) y correr `make release`. Windows no tiene binario propio: se usa el de Linux en WSL.

## Estructura del proyecto

```
lsvp-pupil/
├── cmd/
│   ├── lsvp-pupil/          # main
│   ├── validate/           # valida todos los ejercicios
│   └── genfixtures/        # genera datos de práctica deterministas (semilla fija)
├── internal/
│   ├── content/            # carga de módulos y ejercicios YAML, validación de esquema
│   ├── quiz/               # evaluación de preguntas teóricas (puras, sin I/O)
│   ├── sandbox/            # creación del entorno, ejecución de comandos, límites y bloqueo
│   │   ├── sandbox.go      # interfaz Sandbox y lógica común (guard, reinicios, cierre)
│   │   ├── backend.go      # ParseBackend, Resolve (auto) y detección de bwrap/podman/docker
│   │   ├── tempdir.go      # directorio temporal, passwd/group propios y borrado seguro
│   │   ├── dir.go          # backend: directorio temporal (fallback)
│   │   ├── bwrap.go        # backend: bubblewrap (Linux, recomendado)
│   │   ├── container.go    # backend: docker/podman
│   │   ├── shell.go        # proceso bash persistente + protocolo de marcadores
│   │   ├── procattr_*.go   # grupo de procesos propio (y Pdeathsig en Linux)
│   │   ├── tree.go         # árbol de archivos del sandbox (F3)
│   │   ├── process.go      # procesos del setup, programas interactivos y regla de kill
│   │   └── guard.go        # lista de comandos bloqueados
│   ├── checks/             # validadores: fs, output, exit_code, cwd, pattern, process, script
│   ├── scoring/            # puntos, combos, rangos (funciones puras)
│   ├── session/            # orquesta una partida
│   ├── storage/            # progreso y ranking
│   └── ui/
│       ├── app.go
│       ├── menu.go
│       ├── nav.go          # mensajes de navegación entre pantallas
│       ├── modules.go      # lista de módulos del modo Lecciones
│       ├── lesson.go       # lleva una partida: serie de ejercicios (quiz o práctica) con una sesión
│       ├── hud.go          # HUD, ticks y contexto compartido por las pantallas de ejercicio
│       ├── quiz.go         # pantalla de preguntas teóricas
│       ├── terminal.go     # pantalla de práctica: prompt, salida, historial
│       ├── editor.go       # textarea para escribir scripts (módulo 9)
│       ├── results.go
│       ├── about.go        # «Acerca de»: créditos al LSVP-UAMI, versión, backend y ruta del progreso
│       ├── choice.go       # lista corta de opciones (duración de Contrarreloj)
│       ├── modes.go        # modos de juego y llaves del ranking
│       ├── leaderboard.go
│       ├── strings.go      # todos los textos visibles
│       └── styles.go
├── content/
│   ├── embed.go            # go:embed de modules.yaml y exercises/ (la carga está en internal/content)
│   ├── modules.yaml        # orden, títulos, fuente y requisitos de desbloqueo
│   └── exercises/          # un directorio por módulo (campo dir de modules.yaml)
├── fixtures/               # archivos de práctica generados (taller/, taller2/) + embed.go
├── container/Containerfile # imagen del backend container
├── Makefile
└── CLAUDE.md
```

### Regla de arquitectura

`quiz`, `checks`, `scoring` y `content` no importan `ui` ni Bubble Tea. `sandbox` no conoce la UI: recibe un comando y devuelve un resultado.

## Módulos (basados en el curso)

| # | Módulo | Sección del curso | Tipo principal | Temas |
|---|--------|-------------------|----------------|-------|
| 1 | Introducción | Día 1 · 1.2 | Teoría | Qué es un SO, kernel, terminal, emulador de terminal, intérprete (sh, bash, zsh, fish), distribuciones, GUI vs CLI, historia (Multics, Unix, GNU, Linux, GNU/Linux) |
| 2 | Sistema de archivos | Día 1 · 1.3 | Teoría + práctica | "Todo es un archivo", componentes (namespace, API, permisos, implementación), árbol único, `.` y `..`, rutas absolutas y relativas, tipos de archivo, enlaces, jerarquía de directorios (`/bin /boot /dev /etc /home /lib /mnt /proc /root /tmp /usr /var`) |
| 3 | SSH | Día 1 · 1.4 | Teoría + patrón | Para qué sirve, cifrado, fingerprint, llaves pública/privada, `ssh usuario@host`, openssh vs putty |
| 4 | Comandos básicos | Día 2 · 2.1 | Práctica | Prompt `$` vs `#`, sintaxis `comando [opciones] [argumentos]`, `man` (buscar con `/`, `n`, `N`, `q`), `pwd ls cd mkdir touch cp mv rmdir rm cat less find clear`, historial |
| 5 | Instalación de software | Día 2 · 2.2 | Teoría + patrón | Compilación (gcc, make, configure), paquetes `.deb`/`.rpm`, `dpkg`, `rpm`, repositorios, `apt`, `dnf`, `pacman` |
| 6 | Procesos | Día 3 · 3.1 | Teoría + práctica | PID, PPID, UID, estado, prioridad, `/proc`, `ps aux`, `htop`, `&`, señales INT/TERM/KILL, `kill`, `kill -s` |
| 7 | Redirección y pipes | Día 3 · 3.1 | Práctica | STDIN/STDOUT/STDERR, `< > >> 2> 2>>`, `|` |
| 8 | Filtrado | Día 3 · 3.2 | Práctica | `wc -l -w -c`, `grep -i -v`, `head`/`tail -n`, `cut -d -f`, `sort -t -k -n -r`, `uniq -c -d -u`, combinaciones con pipes sobre `datos.csv` |
| 9 | Scripts | Día 4 · 4.1 | Práctica (scripts) | Shebang, comentarios, `bash script.sh` vs `./script.sh` y permiso de ejecución, nano y vim básicos, variables, `read`/`read -p`, `$(...)`, argumentos `$0 $1 $# $@ $$ $?`, `if/elif/else`, comparaciones (`-eq -ne -gt -lt -ge -le == != -f -d`), `for`, for estilo C, `while`, `(( ))`, códigos de salida, `&&`, `||`, redirección en scripts |
| 10 | Examen final | Todo | Mixto | Ejercicios aleatorios de todos los módulos, sin pistas |

Reglas:
- Mínimo 10 ejercicios por módulo; los módulos teóricos (1, 3, 5) mínimo 15 preguntas.
- Un módulo se desbloquea al completar el anterior con rango C o superior. Los módulos 1–3 están abiertos desde el inicio.
- Si el curso se actualiza, ajustar esta tabla y `content/modules.yaml` juntos.

## Archivos de práctica (fixtures)

Generados por `cmd/genfixtures` con semilla fija, para que las respuestas esperadas sean deterministas. Imitan la estructura del curso:

```
taller/
├── archivo.txt
├── dia-1/
│   ├── Dewey.txt
│   └── frases.txt
├── eje1.md
├── eje_backup.log
├── ejeDatos.txt
├── ejemplo.txt
├── eje_practica.txt
├── ejes.txt
├── notas_eje1.txt
└── .oculto              # para practicar ls -a (no está en el curso)

taller2/dia3/redireccionYPipes/red_pipes/
├── datos.csv
├── poesia_artificial
└── poesia_redireccionada
```

- `datos.csv`: CSV separado por comas con encabezado y ~800 registros de criaturas ficticias (no copiar un dataset existente). Columnas: `id,nombre,tipo1,tipo2,total,hp,ataque,defensa,atq_esp,def_esp,velocidad,generacion,legendario`. Debe permitir preguntas como: número de registros, cuántos legendarios, tipo más frecuente, tipos sin repetir, líneas que contienen cierta palabra. Incluir valores `Fuego`, `Agua`, etc. con mayúsculas variadas para practicar `grep -i`.
- Textos (`Dewey.txt`, `frases.txt`, `poesia_artificial`): textos originales en español, varias líneas.
- Nombres con prefijo `eje` para practicar `find -name 'eje*.txt'`.
- `datos.csv` tiene exactamente 800 registros (así `tail -n 800` deja fuera solo el encabezado, como en el curso). `legendario` vale `Verdadero` o `Falso`. Agua es claramente el tipo más frecuente. Algunos nombres llevan un tipo en minúsculas (`Tlazufuego`) para que `grep agua` y `grep -i agua` den resultados distintos.
- `make fixtures` escribe en `fixtures/` y solo toca `taller/` y `taller2/`. Un test falla si los archivos del repo no coinciden con lo que genera el código: tras cambiar el generador, correr `make fixtures` y agregar los cambios.

## Sandbox (ejecución de comandos)

Los ejercicios prácticos ejecutan bash real. La seguridad del equipo del usuario es prioritaria.

### Backends (flag `--sandbox`)

1. `bwrap` (predeterminado en Linux si `bwrap` está instalado): `--unshare-all`, `/usr` y `/bin` en solo lectura, el directorio del ejercicio como único lugar escribible, `/proc` propio, sin red.
2. `container`: docker o podman con una imagen Debian mínima (incluye `coreutils`, `grep`, `procps`, `findutils`, `less`, `man-db`, `nano`, `vim-tiny`). Recomendado en macOS.
3. `dir`: directorio temporal sin aislamiento real. Solo como último recurso; mostrar un aviso visible en la UI.

`--sandbox auto` (predeterminado) elige en ese orden el primero que funcione. Si se pide uno explícitamente y no está disponible, la app y `validate` terminan con un error claro en vez de caer en `dir`. La disponibilidad se prueba una vez: `bwrap` debe poder crear namespaces (algunas distribuciones los restringen) y `container` necesita podman (preferido: sin root por omisión) o docker, más la imagen.

Detalles de implementación:
- Directorio temporal: `dir` usa `/tmp/lsvp-pupil-XXXX` como home. `bwrap` y `container` usan `.../home` (montado en `/home/alumno`) y `.../meta` con un `passwd`/`group` donde el uid del usuario se llama `alumno`, montados sobre `/etc`. Así `whoami`, `ls -l` y el prompt muestran `alumno` y no el usuario real.
- `bwrap`: además de lo anterior, `--die-with-parent`, `--new-session` (sin inyección de teclas en la terminal real), `--cap-drop ALL`, `--hostname lsvp-pupil`, `/etc` del equipo en solo lectura, `/dev` mínimo, `/tmp` propio y `--remount-ro /`. Matar a bwrap mata a todo su espacio de procesos.
- `container`: `docker|podman run --rm -i` con `--network none`, `--read-only`, `--tmpfs /tmp`, `--cap-drop ALL`, `no-new-privileges`, `--pids-limit 256`, `--memory 512m`, el uid del usuario (`--userns=keep-id` en podman) y `/etc/localtime` del equipo. Matar al cliente no detiene el contenedor, así que al reiniciar o cerrar se hace `rm -f` del contenedor (tiene un nombre aleatorio).
- Imagen: `lsvp-pupil-sandbox:1`, construida con `make image` desde `container/Containerfile` (Debian stable-slim + htop y manuales).
- Límite conocido: `bwrap` no limita el número de procesos (no usa cgroups); una fork bomb que esquive a guard (por ejemplo dentro de un script) afectaría al equipo. `container` sí la limita.

### Reglas comunes a todos los backends

- Cada ejercicio parte de una **copia nueva** de sus fixtures en un directorio temporal. `HOME` apunta a ese directorio.
- Prompt simulado: `alumno@lsvp-pupil:~/taller$`.
- Entorno limpio: `HOME`, `PATH=/usr/local/bin:/usr/bin:/bin`, `USER=alumno`, `TERM=dumb` y `LC_ALL=C.UTF-8`. El locale es fijo para que `sort`/`uniq` den lo mismo en cualquier equipo.
- Límite de 5 s por comando (configurable por ejercicio), 64 KB de salida mostrada, sin acceso a red.
- `guard.go` rechaza antes de ejecutar (toda la línea: si una parte se bloquea, no se ejecuta nada): `sudo`, `su`, `shutdown`, `reboot`, `mkfs*`, `dd of=/dev/*`, fork bombs, rutas absolutas fuera del sandbox en comandos destructivos (`rm`, `mv`, `cp`, `chmod`, `chown`, `>`), y `rm -rf /` o `~` en backend `dir`. Mostrar un mensaje educativo en vez de ejecutar.
  - Implementación: un lexer simplificado de bash parte la línea en comandos simples (respeta comillas, `\`, `;`, `&&`, `|`, subshells, redirecciones) y revisa también `$(...)`, `` `...` ``, `bash -c`, `eval` y envoltorios como `env`, `nohup`, `timeout`, `xargs`. Sigue los `cd` de la misma línea para resolver rutas relativas (`cd / && rm -rf usr`).
  - Comandos destructivos revisados: `rm rmdir mv cp chmod chown chgrp ln truncate shred tee unlink install dd mkdir touch`, y `find` con `-delete`/`-exec`. Cualquier ruta suya fuera del home del sandbox se bloquea (también `cp /etc/x .`), igual que las redirecciones `>`/`>>`/`2>`/`&>` fuera del home (salvo `/dev/null`, `/dev/std*`, `/dev/tty`).
  - En el backend `dir` guard es la única protección, así que además bloquea lo que no puede comprobar: argumentos con `$var`/`$(...)` en comandos destructivos, `xargs rm` y rutas relativas tras un `cd -` o `cd $X`.
  - `kill` en el backend `dir`: solo se permite hacia `%N`, `$!` o PIDs que descienden de la shell del sandbox (se sigue el PPID en `/proc`). `pkill`, `killall`, `kill -1`, PIDs negativos o ajenos se bloquean (`RuleForeignProcess`): sin aislamiento podrían terminar el navegador o la terminal del usuario. En `bwrap`/`container` el espacio de procesos propio ya lo impide.
  - `guard` devuelve un código de regla (`RulePrivilege`, `RuleOutsidePath`...) y un detalle; los mensajes viven en `ui/strings.go`.
  - Límite conocido: es de mejor esfuerzo. Un script (`./x.sh`, `bash x.sh`) o `source` pueden esconder cualquier comando. El aislamiento real lo dan `bwrap` y `container`.
- Al salir del ejercicio se destruye el sandbox y se matan todos los procesos hijos.

### Shell persistente

Un proceso `bash --noprofile --norc` por ejercicio, para que `cd`, variables y procesos en segundo plano persistan entre comandos. Tras cada comando se escribe un marcador para separar la salida y leer el código de salida y el directorio actual:

```
<comando>
printf '\n__LDJ_END__%d__%s\n' "$?" "$PWD"
```

stdout y stderr se capturan por separado (stderr en rojo en la UI).

Detalles de la implementación (`shell.go`):
- Los marcadores llevan un nonce aleatorio por shell (`__LDJ_END_<nonce>__ <código> <pwd>` en stdout y `__LDJ_ERR_<nonce>__` en stderr), para que un `echo` del usuario no los imite.
- El comando llega en un heredoc con delimitador secreto y se ejecuta con `eval "$cmd" < /dev/null`. Así una comilla sin cerrar da error de sintaxis en vez de dejar a bash esperando más líneas, y `cat` o `read` sin argumentos no se comen el marcador.
- Fin de la shell: se detecta con `Process.Wait` y no con `cmd.Wait`, porque un `sleep &` hereda stdout/stderr y los mantiene abiertos. Al terminar la shell se mata al resto de su grupo para cerrar los pipes.
- Tiempo agotado: se mata el grupo de procesos completo, se reinicia la shell en el último directorio conocido y el resultado sale con `TimedOut`, código 124 y `Restarted`. Si el comando termina la shell (`exit`, `set -e` + fallo), también se reinicia. En ambos casos se pierden las variables.
- La salida se corta en 64 KB por flujo (`Truncated`), pero se sigue leyendo hasta el marcador.
- `Close` mata el grupo de procesos (incluidos los de segundo plano) y borra el directorio. Antes restaura permisos por si hubo `chmod 000`, sin seguir enlaces simbólicos, y se niega a borrar algo que no sea un `lsvp-pupil-*` directo de `$TMPDIR`.

### Detalles de la pantalla de práctica

- El sandbox (y la corrida de referencia para `same_as_solution`) se prepara en segundo plano; el tiempo empieza a correr cuando está listo.
- `F2` crea un sandbox nuevo (archivos y setup como al inicio) y funciona también con un comando en curso. El tiempo no se reinicia. Los resultados que lleguen del sandbox anterior se descartan.
- `clear` y `Ctrl-L` limpian la terminal simulada sin ejecutar nada.
- La salida no pasa por una TTY: `ls` lista en una columna, como cuando se usa en un pipe.
- La entrada se desplaza horizontalmente; si el prompt deja menos de 25 columnas (directorios como `taller2/dia3/redireccionYPipes/red_pipes`), va en su propia línea.
- Al salir del módulo, al terminar o con `F10` se cierra el sandbox. `main` además atiende SIGHUP/SIGTERM (cerrar la ventana de la terminal) para no dejar directorios temporales.

### Programas interactivos

`less`, `man`, `htop`, `top`, `nano`, `vim`, `vi`: se ejecutan suspendiendo la TUI con `tea.ExecProcess` dentro del sandbox, y al terminar se vuelve al ejercicio. El tiempo del ejercicio sigue corriendo.

- Se detectan con `sandbox.IsInteractive` (cualquier comando de la línea, también en un pipe: `cat x | less`). También `more`, `view`, `vimtutor` y `watch`.
- Pasan por guard. Corren en un proceso aparte con la TTY real, en el directorio actual de la shell, con los mismos archivos. No comparten variables con la shell persistente.
- `dir`: `bash -c`. `bwrap`: `nsenter` a los namespaces del sandbox en curso, así `htop` y `ps` ven sus procesos (con `--root --wd` y el `cd` adentro, porque `--wd=RUTA` se abriría fuera del sandbox); sin `nsenter`, otro `bwrap` con los mismos archivos. `container`: `docker|podman exec -it`.
- Al volver se evalúan los checks como con cualquier comando (sirve para ejercicios con nano o vim); la salida no se captura.

### Comandos que no se ejecutan

Instalación de paquetes (`apt`, `dnf`, `pacman`, `dpkg`, `rpm`) y `ssh` no se ejecutan de verdad. Esos ejercicios usan el validador `pattern` (el comando escrito se compara con expresiones aceptadas, sin ejecutarlo).

## Formato de ejercicios

### Teoría

```yaml
id: fs-012
module: 2
kind: quiz
format: multiple_choice        # multiple_choice | true_false | short_answer | multi_select
difficulty: 1                  # 1-5
source: https://lsvp-uami.github.io/mdbook-curso-linux-0/dia_1/03_sistemas_archivos.html#Árbol-del-sistema
question: |
  Instalaste un servidor web y quieres cambiar su configuración. ¿En qué directorio buscas?
options: ["/etc", "/var", "/boot", "/tmp"]
answer: "/etc"
explanation: |
  /etc guarda la configuración del sistema y de las aplicaciones instaladas.
time_limit_sec: 20
hints:
  - "Piensa en la palabra 'configuración'."
```

Reglas por formato (las revisa `internal/content`):

- `multiple_choice`: `options` (≥ 2, sin repetir) y una `answer` que esté en `options`.
- `multi_select`: `options` y `answer` como lista; se acierta solo con el conjunto exacto.
- `true_false`: sin `options`; `answer: "verdadero"` o `"falso"`.
- `short_answer`: sin `options`; `answer` es la respuesta que se muestra y `accepted: [...]` agrega variantes. Se compara sin mayúsculas ni espacios extremos.

Las opciones se barajan al mostrarse (salvo `true_false`), así que no escribir opciones como «todas las anteriores».

Archivos: un archivo YAML puede tener varios ejercicios separados por `---`. Los archivos se leen en orden alfabético y ese es el orden del modo Lecciones, así que llevan prefijo numérico (`01-conceptos.yaml`). Campos desconocidos son error.

`modules.yaml`: `id`, `title`, `dir`, `kind` (`theory` mín. 15 ejercicios, `mixed`/`practice` mín. 10, `exam` sin ejercicios propios), `open` (abierto desde el inicio) y `source`. `make validate` marca como «pendiente» un módulo sin ejercicios y falla si tiene menos del mínimo.

### Práctica

```yaml
id: basicos-014
module: 4
kind: practice
difficulty: 2
source: https://lsvp-uami.github.io/mdbook-curso-linux-0/dia_2/04_comandos_basicos.html#cómo-se-puede-mover-un-directorio-o-archivo-mv
fixture: taller
start_dir: taller/dir1
setup:                          # comandos previos opcionales (el usuario no los ve)
  - mkdir -p ../dir3 && touch file.txt
instructions: |
  Desde dir1, mueve file.txt al directorio dir3.
checks:
  - type: fs
    exists: [taller/dir3/file.txt]
    absent: [taller/dir1/file.txt]
solution: "mv file.txt ../dir3"
par_chars: 19                   # longitud de la solución de referencia
max_attempts: 3
time_limit_sec: 60
hints:
  - "mv origen destino"
  - "El directorio padre se escribe .."
```

### Tipos de validador (`checks`)

| type | Qué valida |
|------|-----------|
| `fs` | `exists`, `absent`, `is_dir`, `is_file`, `executable`, `content_equals`, `content_contains`, `line_count` |
| `output` | stdout del último comando. `mode: exact | trimmed | regex | same_as_solution`. `same_as_solution` ejecuta `solution` en un sandbox limpio y compara salidas (preferido para filtrado) |
| `exit_code` | código de salida esperado |
| `cwd` | directorio actual tras el comando (para ejercicios de `cd`) |
| `process` | un proceso lanzado en `setup` (p. ej. `watch uptime &`) ya no existe, o recibió cierta señal |
| `pattern` | el texto del comando coincide con alguna regex de `accepted` (no se ejecuta) |
| `script` | ejercicio de script: ver abajo |

Un ejercicio se considera resuelto cuando **todos** sus checks pasan.

Campos exactos (las rutas son relativas al home del sandbox, sin `/` inicial ni `..` que salga de él):

```yaml
- type: fs
  exists: [taller/a]            # también absent, is_dir, is_file, executable
  content_equals: {taller/a.txt: "texto exacto\n"}
  content_contains: {taller/a.txt: "fragmento"}
  line_count: {taller/a.txt: 3} # cuenta saltos de línea, como wc -l
- type: output
  mode: regex                   # exact | trimmed | regex | same_as_solution
  expected: '(?m)^d.*dia-1$'    # no se usa con same_as_solution
- type: exit_code
  code: 0
- type: cwd
  path: taller/dia-1            # "." es el home
- type: pattern
  accepted:                     # regex; cada una debe cubrir la línea completa
    - '(sudo )?apt(-get)? install( -y)? cowsay'
- type: process
  process: sleep                # nombre (comm) de un programa que el setup lanzó con &
  signal: KILL                  # opcional: HUP INT QUIT KILL TERM USR1 USR2
```

- Valores predeterminados: `max_attempts: 3` y `par_chars` = longitud de `solution` sin espacios repetidos.
- `same_as_solution`: la solución corre en otro sandbox limpio al preparar el ejercicio. Antes de comparar, el home de cada sandbox se reemplaza por `~` y se ignoran los saltos de línea finales. Si la solución no produce salida, el check nunca pasa.
- La salida de `find` depende del orden del disco: para ella usar varios checks `regex` en vez de `same_as_solution`. Lo mismo con `ls -l`, que incluye la hora.
- Con `same_as_solution`, evitar ordenar líneas completas por una columna con empates (p. ej. `sort -k5 -n` sobre `datos.csv`): `-k5nr` y `-k 5 -n -r` desempatan distinto y una respuesta correcta fallaría. Mejor extraer la columna antes de ordenar.
- `cmd/validate/alternatives_test.go` guarda respuestas correctas distintas a la solución (`sed -n 5,10p`, `grep -c`, `sort -u`...). Al escribir un ejercicio con `same_as_solution` o `regex`, agregar ahí las variantes que un alumno escribiría.
- Cada comando de `setup` debe terminar con código 0; si falla a propósito (p. ej. `ls noexiste 2> errores.txt`), agregar `|| true`.
- `pattern`: el comando **no se ejecuta**. Se normaliza (sin espacios extremos y un solo espacio entre palabras) y cada regex se ancla a la línea completa (`^(?:…)$`), así `sudo apt install x && rm -rf /` no coincide con `sudo apt install x`. Un ejercicio con checks `pattern` solo puede tener checks `pattern`, no lleva `fixture` ni `setup`, no crea sandbox y no pasa por guard (`sudo apt install` es una respuesta válida). La regla de exploración también aplica: `ls` no cuenta como intento. En las direcciones de ejemplo usar dominios `.example` o IPs de documentación (`192.0.2.x`).
- `process`: pasa si el proceso ya no corre y, si se indica `signal`, si terminó por esa señal. Después del setup el sandbox guarda los trabajos en segundo plano (`jobs -p`). Para revisarlos corre un comando oculto en la shell: `ps` dice si sigue vivo (un zombi cuenta como terminado) y `wait PID` da 128+señal. Si la shell se reinició, esos procesos murieron con ella y no se conoce la señal.
  - Ojo: en una shell no interactiva los trabajos en segundo plano ignoran INT. Para ejercicios de INT, lanzar el proceso con `env --default-signal=INT programa &`.
  - `watch` no funciona sin una TTY, así que no sirve para el setup; usar `sleep`.
- `make validate`, para cada práctica: la solución no debe estar bloqueada por guard ni agotar el tiempo, debe pasar todos los checks y, en un sandbox recién creado (sin escribir nada), los checks deben **fallar**. En las prácticas `pattern`: la solución debe coincidir y una línea vacía no.

### Scripts (módulo 9)

```yaml
kind: script
filename: argumentos.sh
instructions: |
  Crea un script que muestre su nombre, el primer y segundo argumento,
  el total de argumentos y todos los argumentos.
tests:
  - args: ["hola", "1234", "verde"]
    stdout_regex: "argumentos\\.sh[\\s\\S]*hola[\\s\\S]*1234[\\s\\S]*3[\\s\\S]*hola 1234 verde"
  - args: ["a", "b", "c", "d"]
    stdout_regex: "[\\s\\S]*4[\\s\\S]*"
  - stdin: "Ana\n20\nComputación\n"   # para scripts con read
    stdout_contains: "Ana"
  - args: ["ejemplo.txt"]
    exit_code: 0                        # código de salida esperado (opcional)
    files: {"salida.txt": "4\n"}         # contenido exacto tras el test, relativo a start_dir (opcional)
require_shebang: true
solution_file: solutions/argumentos.sh
```

El usuario escribe el script en el editor integrado (`textarea`) o con `F4` abre `$EDITOR`/nano/vim en el sandbox. `F5` ejecuta los tests.

Detalles de la implementación (`ui/editor.go`, `checks/script.go`):
- El script vive en `start_dir/filename` dentro del sandbox. Si el `setup` deja ahí un archivo, se carga como borrador.
- Cada test corre desde `start_dir` como `bash ./filename 'arg1' 'arg2' < <(printf '%s' 'stdin')` (sin `stdin`: `< /dev/null`), con los argumentos entre comillas simples. Los tests de un ejercicio corren en orden en el mismo sandbox. `stdout_regex` busca en cualquier parte de la salida (usar `\A`/`\z` para anclar) y `stdout_contains` es una subcadena. `exit_code` compara el código de salida y `files` el contenido exacto de archivos después del test. Cada test lleva al menos una de esas cuatro expectativas. Como los tests corren en orden en el mismo sandbox, dos tests seguidos sobre el mismo archivo detectan `>>` donde se pedía `>`.
- Con `require_shebang`, si la primera línea no empieza con `#!`, no se corre nada y cuenta como intento fallido.
- Cada `F5` que no pasa todos los tests es un intento fallido. `F4` abre `$LSVP_EDITOR` o `$EDITOR` solo si es nano, vim o vi (lo que existe en el sandbox); si no, nano. Tab inserta 4 espacios.
- `solution_file` es relativo al directorio del YAML (`content/exercises/09-scripts/solutions/`). `par_chars` se calcula de la solución.
- Al fallar, la UI muestra la salida del script y un motivo para el alumno ("se esperaba que mostrara «…»"), nunca la regex.
- `make validate`: la solución pasa todos los tests y un script con solo el shebang no. `cmd/validate/scripts_test.go` guarda scripts correctos alternativos.
- `cmd/validate/scripts_test.go` también guarda scripts **incorrectos** típicos que deben fallar (`>>` en vez de `>`, `wc -l archivo` con el nombre, olvidar `exit 1`...).

### Reglas del contenido

- Todo ejercicio práctico debe tener `solution` (o `solution_file`) y pasar `make validate`.
- Todo ejercicio debe tener `source` apuntando a la sección del curso.
- Textos en español, tono cercano, sin copiar literalmente el curso.
- En `question` y `explanation`, los saltos de línea simples se unen al mostrarse (el texto se ajusta al ancho de la terminal); una línea en blanco separa párrafos.

## Puntuación

### Por ejercicio

```
base = 100 × difficulty

bonus_tiempo = base × 0.5 × max(0, 1 − t_usado / time_limit)

Teoría:
  bonus_precision = base × 0.5 si acierta al primer intento, 0 en otro caso
  (quiz tiene 1 intento, salvo modo Lecciones donde se permiten 2 con −50% de base)

Práctica:
  bonus_precision = base × 0.5 × (1 − (intentos_fallidos / max_attempts))
  bonus_elegancia = base × 0.1 si len(comando) ≤ par_chars × 1.2   (sin contar espacios repetidos)

penalizaciones:
  - pista usada:         −15% de base por pista
  - comando bloqueado por guard: −10 puntos (solo la primera vez, con explicación)

subtotal = max(0, base + bonuses − penalizaciones)
puntos   = subtotal × multiplicador_combo
```

- Tiempo agotado o intentos agotados: 0 puntos, se rompe el combo y se muestra la solución con explicación.
- Un intento fallido en práctica es un comando que termina sin que los checks pasen **y** que modificó algo o produjo salida incorrecta; comandos de exploración (`ls`, `pwd`, `cat`, `cd` sin cumplir el objetivo) **no** cuentan como intento fallido. La lista de comandos de exploración vive en `scoring/exploration.go`.
  - Regla concreta: una línea es exploración si todos sus comandos están en la lista, no tiene `>` y **ninguno aparece en la solución**. Así, si la solución es `cat Dewey.txt`, un `cat frases.txt` sí cuenta como intento fallido, y un `ls` no.
  - Un comando bloqueado por guard no cuenta como intento (no se ejecutó).

### Combo

+0.1 por ejercicio resuelto sin pistas y al primer intento. Tope ×2.0. Se reinicia con pista, fallo, tiempo agotado o salto.

### Cómo se aplican las fórmulas (`internal/scoring`, `internal/session`)

- El ejercicio usa el multiplicador vigente **antes** de él; después el combo sube o se reinicia. El primer acierto vale ×1.0.
- Quiz en Lecciones acertado en el segundo intento: sin bono de precisión y una penalización de 50% de base (el bono de tiempo se calcula sobre la base completa).
- La penalización de guard (−10) se aplica una sola vez por partida.
- Máximo teórico sin combo por ejercicio: teoría 2.0 × base, práctica 2.1 × base. Con combo el porcentaje puede pasar de 100%.
- Puntos por ejercicio: se redondean al entero más cercano después de aplicar el combo.
- «Temas a repasar»: se agrupan los fallos (no resuelto o no resuelto al primer intento) por `source`, y se muestran los 3 con más fallos con el nombre del ancla de la sección.
- «Nuevo récord» se mostrará cuando exista `storage` (fase 10).

### Puntaje final y rango

```
porcentaje = puntaje_total / máximo_teórico_sin_combo × 100
```

| Rango | Porcentaje |
|-------|-----------|
| S | ≥ 95% |
| A | ≥ 85% |
| B | ≥ 70% |
| C | ≥ 50% |
| D | < 50% |

La pantalla final muestra: puntaje total, rango, tiempo total, precisión (aciertos al primer intento / total), combo máximo, **desglose por módulo** (barras) y los **temas a repasar** (módulos o etiquetas con más fallos, con enlace a la sección del curso). Indica si es nuevo récord.

## Modos de juego

- **Lecciones**: módulo a módulo, con pistas y explicaciones. Guarda progreso.
- **Contrarreloj**: 60/120/300 s globales, ejercicios aleatorios de módulos desbloqueados.
- **Quiz rápido**: solo teoría, 20 preguntas.
- **Examen final**: 25 ejercicios mixtos (≈40% teoría, 60% práctica), sin pistas, da el rango "oficial".

Cómo se implementaron (`session/pick.go`, `ui/lesson.go`, `ui/app.go`):
- Todos los modos usan el mismo runner (`lessonModel`) con un `gameMode`. Solo Lecciones da pistas y un segundo intento en quiz.
- **Contrarreloj**: ejercicios de los módulos desbloqueados, sin scripts (tardan minutos), en orden aleatorio y repitiéndose si se acaban. Cada ejercicio conserva su propio límite. Al acabarse el tiempo total termina la partida; el ejercicio en curso no cuenta. El porcentaje y el rango se calculan sobre los ejercicios jugados.
- **Quiz rápido**: 20 preguntas de teoría al azar de los módulos desbloqueados (menos si no hay tantas).
- **Examen final**: es el módulo 10, así que se desbloquea con rango C o más en el módulo 9 (o con `--unlock-all`). Usa ejercicios de todos los módulos: 10 de teoría y 15 de práctica, con a lo más 2 scripts. Si falta de un tipo, completa con el otro.
- Ranking: un top 10 por modo; cada duración de Contrarreloj es un modo aparte (`contrarreloj-60`...). Lecciones no entra al ranking: su récord es el mejor rango del módulo.

## Interfaz

### Pantalla de práctica

1. **HUD**: módulo/ejercicio, temporizador (amarillo < 30%, rojo < 10%), puntos, combo, intentos restantes.
2. **Instrucciones**, con indicador del backend de sandbox activo.
3. **Terminal simulada** (viewport con scroll): prompt, comandos anteriores, stdout normal, stderr en rojo.
4. **Entrada**: `textinput` con historial (flechas arriba/abajo) y `Ctrl-L` para limpiar como en el curso.
5. **Ayuda**: `F1` pista, `F2` reiniciar sandbox, `F3` ver árbol de archivos del sandbox, `F10` salir.

### Pantalla de quiz

Pregunta, opciones seleccionables con flechas o números, temporizador. Tras responder: correcto/incorrecto, explicación y enlace a la sección del curso.

Tamaño mínimo 80×24; si es menor, mostrar aviso. Colores solo en `ui/styles.go`, textos solo en `ui/strings.go`.

Teclas globales: `Ctrl-C` y `F10` salen desde cualquier pantalla. `q` solo sale desde el menú (en la pantalla de práctica es texto normal).

## Persistencia

`~/.config/lsvp-pupil/` (con `os.UserConfigDir()`):
- `progress.json`: módulos desbloqueados, mejor rango por módulo, ejercicios resueltos, estadísticas de fallos por etiqueta.
- `scores.json`: top 10 por modo.

Escritura atómica (temporal + rename). JSON dañado → respaldar como `.bak` y empezar de cero.

Detalles (`internal/storage`):
- `progress.json`: `modules` (mejor rango, mejor porcentaje y veces completado por módulo), `solved` (ids resueltos alguna vez), `misses` (fallos por `source`, es decir, por sección del curso) y `exam_rank`.
- Un módulo cuenta como completado solo si se llega a la pantalla de resultados (salir con esc no cuenta). Su "mejor" es el de mayor porcentaje.
- Nuevo récord: el primer lugar del modo con más puntos que el anterior (empatar no cuenta). Si no es récord pero entra al top 10, se muestra el lugar.
- Si no se puede leer o escribir, la app sigue funcionando y lo avisa: en el menú al iniciar, o en la pantalla de resultados al guardar.
- La pantalla de resultados se desplaza con ↑/↓ cuando no cabe (el examen puede tener 9 módulos en el desglose).

## Testing

- **quiz**: normalización de respuestas, multi_select, casos límite.
- **checks**: cada validador con directorios temporales reales.
- **sandbox**: protocolo de marcadores (salida con saltos de línea, sin salto final, stderr), persistencia de `cd` y variables, timeout, limpieza de procesos, `guard` con tabla de comandos permitidos/bloqueados.
- **scoring**: fórmulas y casos límite (tiempo 0, intentos agotados, combo en tope, penalizaciones que darían negativo).
- **content**: `make validate` carga todo el YAML, verifica esquema, que cada `source` apunte al dominio del curso y que cada solución pase sus checks.
- Tests de sandbox que requieren `bwrap` o docker se saltan con `t.Skip` si no están disponibles. Las pruebas de comportamiento común corren en todos los backends disponibles (`forEachBackend`); `isolation_test.go` comprueba el aislamiento real de `bwrap` y `container`.

## Convenciones de código

- `gofmt` obligatorio; identificadores en inglés, textos al usuario en español.
- Errores con contexto: `fmt.Errorf("ejecutar comando en sandbox %s: %w", id, err)`.
- Sin estado global; dependencias explícitas.
- Nunca ejecutar comandos del usuario fuera del paquete `sandbox`.
- Comentarios solo donde el comportamiento de bash o de la herramienta GNU no es obvio.

## Hoja de ruta

- [x] **Fase 0**: esqueleto, Makefile, menú vacío
- [x] **Fase 1**: quiz engine + pantalla de quiz + módulo 1 completo
- [x] **Fase 2**: scoring + resultados + puntaje final con rango y desglose
- [x] **Fase 3**: genfixtures + sandbox backend `dir` + shell persistente + guard
- [x] **Fase 4**: pantalla de práctica + validadores `fs`, `cwd`, `output` + módulos 2 y 4
- [x] **Fase 5**: backends `bwrap` y `container`
- [x] **Fase 6**: validador `pattern` + módulos 3 y 5
- [x] **Fase 7**: validador `process` + programas interactivos + módulo 6
- [x] **Fase 8**: módulos 7 y 8 (redirección, pipes, filtrado con `same_as_solution`)
- [x] **Fase 9**: editor + validador `script` + módulo 9
- [x] **Fase 10**: persistencia, ranking, modos Contrarreloj/Quiz rápido/Examen final
- [x] **Fase 11**: pulido visual, pantalla "Acerca de" con créditos, release

Marcar las casillas al completar cada fase.

## Cómo trabajar en este repo (instrucciones para Claude)

- Trabajar **una fase o subtarea a la vez**; al terminar, resumir qué cambió y proponer el siguiente paso.
- Cualquier cambio en `sandbox` o `guard` requiere tests nuevos y una nota breve en el resumen sobre su impacto en seguridad.
- Para crear ejercicios, consultar la sección correspondiente del curso (usar la versión `print.html`), redactar con palabras propias y rellenar `source`.
- No modificar fórmulas de puntuación, el esquema YAML ni la lista de módulos sin confirmar con el usuario; si cambian, actualizar este archivo.
- Si una decisión no está cubierta aquí, elegir la opción más simple, anotarla en la sección correspondiente y mencionarlo en el resumen.
