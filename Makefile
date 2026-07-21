# html-artifacts — build / install / serve / test
#
# The Go module lives in ./server. The binary is emitted to ./bin/html-artifacts.

BIN_DIR   := bin
BIN       := $(BIN_DIR)/html-artifacts
PORT      ?= 47600
DIR       ?= $(HOME)/.html-artifacts/artifacts

.PHONY: build install serve test vet fmt clean sync

## build: compile the server binary into ./bin
build:
	mkdir -p $(BIN_DIR)
	cd server && go build -o ../$(BIN) .

## test: go vet + go test across the server module
test: vet
	cd server && go test ./...

## vet: go vet across the server module
vet:
	cd server && go vet ./...

## fmt: format the server module
fmt:
	cd server && gofmt -w .

## serve: run the server on 127.0.0.1:$(PORT)
serve: build
	./$(BIN) serve --port $(PORT) --dir $(DIR)

## sync: copy canonical CORE.md + ensure-server.sh + base.html into embed, plugin skill, and adapters
sync:
	mkdir -p skills/html-artifacts/templates adapters/claude-code/templates
	cp instructions/CORE.md skills/html-artifacts/CORE.md
	cp scripts/ensure-server.sh skills/html-artifacts/ensure-server.sh
	cp adapters/claude-code/SKILL.md skills/html-artifacts/SKILL.md
	cp instructions/templates/base.html skills/html-artifacts/templates/base.html
	cp instructions/CORE.md adapters/claude-code/CORE.md
	cp scripts/ensure-server.sh adapters/claude-code/ensure-server.sh
	cp instructions/templates/base.html adapters/claude-code/templates/base.html
	cp instructions/templates/base.html server/embed/base.html   # for the `render` subcommand's embedded template

## install: install the Claude Code adapter (pass ARGS='--local' etc.)
install:
	./install.sh --agent claude $(ARGS)

## clean: remove build output
clean:
	rm -rf $(BIN_DIR)
