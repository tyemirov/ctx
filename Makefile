APP_ROOT := $(shell git -C "$(dir $(abspath $(firstword $(MAKEFILE_LIST))))" rev-parse --show-toplevel)
MPRLAB_GATEWAY_EXECUTABLE ?= mprlab-gateway
MPRLAB_GOVERNOR_EXECUTABLE ?= normalize-mprlab

.PHONY: lint-audit test test-go test-browser lint ci governance-check release publish deploy check-release-build

test: test-go test-browser

test-go:
	go test ./...

test-browser:
	npm test

lint:
	go vet ./...
	staticcheck -checks 'SA*' ./...
	ineffassign ./...
	test -z "$$(gofmt -l $$(git ls-files '*.go'))"

ci: lint test

lint-audit:
	golangci-lint run

governance-check:
	"$(MPRLAB_GOVERNOR_EXECUTABLE)" --repo "$(APP_ROOT)" --check --json

release publish deploy: governance-check
	"$(MPRLAB_GATEWAY_EXECUTABLE)" app-$@ --app-root "$(APP_ROOT)"

check-release-build:
	go test -tags releasebuild ./tests -run '^TestReleaseBinaries$$' -count=1 -v
