MODULE   := github.com/poconnor/slack-cli
BIN      := bin/slack-cli
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: generate build test lint install clean e2e

generate:
	go generate ./...

build: generate
	go build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/slack-cli

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

install: build
	cp $(BIN) $(GOPATH)/bin/slack-cli

clean:
	rm -rf bin/

e2e:
	@echo "TODO: end-to-end tests not yet implemented"
