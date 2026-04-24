.PHONY: build vet test test-race fmt lint no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./... -count=1 -short

test-race:
	go test ./... -race -count=1

fmt:
	gofmt -l -w .

lint:
	@command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "lint: golangci-lint not installed; install from https://golangci-lint.run"

# Definition of Done gates
no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

# Single entry point — warn-mode during transition.
ci-validate-all: fmt vet test no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"
