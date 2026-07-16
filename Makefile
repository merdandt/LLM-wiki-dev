.PHONY: fmt test vet build package verify

fmt:
	test -z "$$(gofmt -l cmd internal)"

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/llm-wiki

package:
	./scripts/package-release.sh

verify: test vet build
