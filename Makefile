.PHONY: build test lint modernize install clean run

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o ./bin/jmake .

test:
	go test -v -race ./...

lint:
	@test -f .golangci.yml || cp "$(HOME)/git/sammcj/mcp-devtools/.golangci.yml" .golangci.yml 2>/dev/null || true
	golangci-lint run ./...

modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test ./...

install: build
	cp ./bin/jmake $(GOPATH)/bin/ 2>/dev/null || cp jmake /usr/local/bin/

clean:
	rm -f ./bin/jmake

run: build
	./bin/jmake $(ARGS)
