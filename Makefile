.PHONY: fmt test vet build verify

fmt:
	test -z "$$(gofmt -l cmd internal)"

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/llm-wiki

verify: test vet build
