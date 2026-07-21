# html-artifacts — build / install / serve / test
#
# The Go module lives in ./server. The binary is emitted to ./bin/html-artifacts.

BIN_DIR   := bin
BIN       := $(BIN_DIR)/html-artifacts
PORT      ?= 7777
DIR       ?= ./artifacts

.PHONY: build install serve test vet fmt clean

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

## install: install the Claude Code adapter (pass ARGS='--local' etc.)
install:
	./install.sh --agent claude $(ARGS)

## clean: remove build output
clean:
	rm -rf $(BIN_DIR)
