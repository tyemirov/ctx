.PHONY: lint-audit test test-go test-browser lint ci

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
