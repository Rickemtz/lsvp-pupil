.PHONY: run build test lint validate fixtures image release clean

# Versión: la etiqueta de git más cercana (p. ej. v1.0.0-3-gabc123); «dev» fuera de un repositorio.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

run:
	go run ./cmd/lsvp-pupil

build:
	go build -ldflags "$(LDFLAGS)" -o bin/lsvp-pupil ./cmd/lsvp-pupil

test:
	go test ./...

lint:
	go vet ./...
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt:"; echo "$$out"; exit 1; fi

validate:
	go run ./cmd/validate

fixtures:
	go run ./cmd/genfixtures

# Imagen del backend container. Usa podman si está instalado; si no, docker.
RUNTIME ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
image:
	$(RUNTIME) build -t lsvp-pupil-sandbox:1 -f container/Containerfile container/

# Binarios para publicar en dist/, con sus sumas SHA-256. Antes corre las mismas revisiones que
# una tarea terminada. Windows no está: se usa desde WSL con el binario de Linux.
release: test lint validate
	rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; out=dist/lsvp-pupil-$(VERSION)-$$os-$$arch; \
		echo "  $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $$out ./cmd/lsvp-pupil || exit 1; \
	done
	cd dist && sha256sum lsvp-pupil-* > SHA256SUMS

clean:
	rm -rf bin dist
