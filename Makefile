# Mockelot local build helpers
# Targets for Debian 13 (webkit2gtk-4.1) development

WAILS := $(HOME)/go/bin/wails
BINARY := build/bin/mockelot

.PHONY: build race dev run deb clean

## build: compile a runnable binary for Debian 13
build:
	$(WAILS) build -tags webkit2_41

## race: compile with Go race detector enabled (for debugging data races)
race:
	$(WAILS) build -tags webkit2_41 -race

## dev: hot-reload dev server (Debian 13)
dev:
	$(WAILS) dev -tags webkit2_41

## run: build then launch
run: build
	./$(BINARY)

## deb: build a native Debian 13 .deb package for local testing
deb:
	./scripts/build-deb-native.sh

## clean: remove build artifacts
clean:
	rm -f $(BINARY) build/bin/mockelot-linux-amd64
