# FM NewGen Faces – build system
APP     := fm-newgen-faces
MODULE  := fmnewgenfaces
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev)
LDFLAGS := -s -w -X $(MODULE)/internal/brand.Version=$(VERSION)

.PHONY: build run test vet fmt lint check clean icon help

build: ## Build for the current platform
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP)$(EXT) .

build-windows: ## Windows GUI (no console) and CLI binaries (run on Windows)
	go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" -o $(APP).exe .
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP)-cli.exe .

run: build ## Build and launch the GUI
	./$(APP)$(EXT)

test: ## Run all tests
	go test ./... -count=1

vet: ## go vet
	go vet ./...

fmt: ## gofmt check
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "run gofmt -w ." && exit 1)

check: fmt vet test ## Everything CI runs

icon: ## Regenerate assets/icon.png
	go run ./assets/gen

clean:
	rm -f $(APP) $(APP).exe

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'
