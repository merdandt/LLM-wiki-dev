.PHONY: test vet build verify

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/llm-wiki

verify: test vet build
