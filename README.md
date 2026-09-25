<div align="center">

# Linuxpupil

**Aprende la terminal GNU/Linux jugando: lecciones, comandos reales en un sandbox, contrarreloj y rangos.**

[Características](#características) · [Inicio rápido](#inicio-rápido) · [Cómo se juega](#cómo-se-juega) · [Sandbox](#sandbox) · [Desarrollo](#desarrollo) · [Créditos](#créditos)

</div>

Linuxpupil es una aplicación de terminal (TUI) para aprender y evaluar el uso de la terminal GNU/Linux. Sigue el temario del curso **[Terminal GNU/Linux Nivel cero](https://lsvp-uami.github.io/mdbook-curso-linux-0/)** del Laboratorio de Supercómputo y Visualización en Paralelo (LSVP) de la UAM Iztapalapa: desde qué es un kernel hasta escribir scripts de bash.

```
  Comandos básicos · 9 de 15   ⏱ 41s   1480 pts  ×1.3   Intentos: 3

  Desde dir1, mueve file.txt al directorio dir3 (que está junto a dir1, dentro
  de taller).
  Sandbox: bwrap

  alumno@lsvp-pupil:~/taller/dir1$ ls
  file.txt
  alumno@lsvp-pupil:~/taller/dir1$ mv file.txt ../dir3
  ✔ ¡Resuelto!  +505 pts (combo ×1.3)
```

## Características

- **153 ejercicios en 9 módulos**: introducción, sistema de archivos, SSH, comandos básicos, instalación de software, procesos, redirección y pipes, filtrado y scripts.
- **Comandos de verdad**: los ejercicios prácticos se resuelven escribiendo bash real sobre archivos de práctica, dentro de un sandbox aislado. La app revisa el resultado (archivos, salida, directorio, procesos), no el texto exacto del comando.
- **Programas interactivos**: `less`, `man`, `htop`, `nano` y `vim` funcionan dentro del sandbox.
- **Editor de scripts** integrado, con pruebas automáticas (argumentos, entrada estándar, salida, código de salida y archivos).
- **Cuatro modos**: Lecciones (con pistas), Contrarreloj (60, 120 o 300 s), Quiz rápido y Examen final.
- **Puntos, combos y rangos** (S, A, B, C, D), desglose por módulo y los temas a repasar con enlace a la sección del curso.
- **Progreso y ranking** guardados en tu equipo. Cada módulo se desbloquea al terminar el anterior con rango C o mejor.

## Inicio rápido

Necesitas **Go 1.24.2 o más reciente**, **bash** y, para el aislamiento recomendado en Linux, **[bubblewrap](https://github.com/containers/bubblewrap)** (`bwrap`).

Desde el directorio del proyecto:

```bash
make build
./bin/lsvp-pupil
```

La terminal debe medir al menos 80×24.

> [!TIP]
> Instala bubblewrap antes de jugar: `sudo apt install bubblewrap` (Debian/Ubuntu), `sudo dnf install bubblewrap` (Fedora) o `sudo pacman -S bubblewrap` (Arch). Sin él, la app usa un modo sin aislamiento y lo avisa en pantalla.

### Opciones

| Opción | Qué hace |
|--------|----------|
| `--sandbox auto\|bwrap\|container\|dir` | Cómo se aíslan los ejercicios prácticos. `auto` (predeterminado) elige el mejor disponible. |
| `--unlock-all` | Abre todos los módulos sin completar los anteriores (útil para docentes). |
| `--version` | Muestra la versión. |

### Plataformas

- **Linux**: la plataforma principal. Con `bwrap` tienes el mejor aislamiento.
- **macOS**: funciona, pero las herramientas BSD (`sed`, `sort`, `find`, `ps`) no son las del curso. Usa el modo contenedor (ver [Sandbox](#sandbox)).
- **Windows**: desde WSL, con el binario de Linux.

## Cómo se juega

Desde el menú eliges un modo. En **Lecciones** avanzas módulo por módulo; los módulos 1 a 3 están abiertos desde el inicio.

| Tipo de ejercicio | Qué haces |
|-------------------|-----------|
| Teoría | Opción múltiple, verdadero/falso, respuesta corta o varias opciones. |
| Práctica | Escribes comandos en una terminal real dentro del sandbox. Explorar con `ls`, `cat` o `cd` no gasta intentos. |
| Comando (SSH, paquetes) | Escribes el comando; se compara con las respuestas aceptadas, pero no se ejecuta. |
| Script | Lo escribes en el editor y lo pruebas con F5. |

**Teclas en la terminal de práctica**

| Tecla | Acción |
|-------|--------|
| `Enter` | Ejecutar el comando |
| `↑` / `↓` | Historial |
| `PgUp` / `PgDn` | Desplazar la salida |
| `F1` | Pista (solo en Lecciones) |
| `F2` | Reiniciar el sandbox (los archivos vuelven a su estado inicial) |
| `F3` | Ver el árbol de archivos |
| `Ctrl-L` | Limpiar la pantalla |
| `Esc` / `F10` | Salir del módulo / salir de la app |

En el editor de scripts: `F5` guarda y prueba, `F4` abre el archivo en nano o vim (respeta `$EDITOR` si es uno de ellos).

**Puntuación.** Cada ejercicio vale 100 × dificultad, más bonos por rapidez, por acertar al primer intento y, en la práctica, por un comando corto. Las pistas restan. Encadenar aciertos sin pistas sube el combo hasta ×2.0. El rango final sale del porcentaje sobre el máximo sin combo: S ≥ 95 %, A ≥ 85 %, B ≥ 70 %, C ≥ 50 %.

**Progreso.** Se guarda en `~/.config/lsvp-pupil/` (`progress.json` y `scores.json`). Si un archivo se daña, se respalda como `.bak` y se empieza de cero.

## Sandbox

Cada ejercicio práctico parte de una copia nueva de los archivos de práctica (`taller/`, `taller2/`, `datos.csv`...), con un límite de 5 s por comando y sin red.

| Backend | Aislamiento | Cuándo usarlo |
|---------|-------------|---------------|
| `bwrap` | Namespaces de Linux: sistema en solo lectura, sin red, procesos propios, usuario `alumno`. | Linux (predeterminado si está instalado). |
| `container` | Contenedor desechable de podman o docker: sin red, raíz de solo lectura, límites de procesos y memoria. | macOS, o Linux sin bubblewrap. Requiere `make image`. |
| `dir` | Ninguno: un directorio temporal. | Último recurso. |

> [!WARNING]
> En el modo `dir`, los comandos corren en tu equipo con tus permisos. La app bloquea lo peligroso más evidente (`sudo`, `rm` fuera del directorio de práctica, `kill` a procesos ajenos, fork bombs...), pero un script puede esconder cualquier comando. Úsalo solo si no puedes instalar bubblewrap ni un contenedor.

Para usar el modo contenedor:

```bash
make image                            # construye lsvp-pupil-sandbox:1 con podman o docker
./bin/lsvp-pupil --sandbox container
```

## Desarrollo

```bash
make run        # ejecutar sin compilar
make test       # pruebas
make lint       # go vet + gofmt
make validate   # revisa el contenido y ejecuta la solución de cada ejercicio en un sandbox
make fixtures   # regenera los archivos de práctica
make release    # binarios para Linux y macOS en dist/, con SHA256SUMS
```

El contenido vive en `content/` como YAML (un directorio por módulo) y se incluye en el binario. `make validate` comprueba el esquema, que cada solución resuelva su ejercicio y que un ejercicio no esté resuelto antes de empezar.

## Créditos

El temario sigue el curso **[Terminal GNU/Linux Nivel cero](https://lsvp-uami.github.io/mdbook-curso-linux-0/)** del **Laboratorio de Supercómputo y Visualización en Paralelo (LSVP)** de la Universidad Autónoma Metropolitana, Unidad Iztapalapa. El material del curso se publica bajo la licencia Apache-2.0 en [LSVP-UAMI/mdbook-curso-linux-0](https://github.com/LSVP-UAMI/mdbook-curso-linux-0).

Las preguntas, explicaciones y archivos de práctica de Linuxpupil se escribieron para esta aplicación a partir de ese temario; no son copia del curso.
