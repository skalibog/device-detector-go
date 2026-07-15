.PHONY: build test test-fixtures vet sync-upstream

build:
	go build ./...

test:
	go test ./...

test-fixtures:
	go test -run TestFixtures -timeout 60m -v .

vet:
	go vet ./...

sync-upstream:
	bash scripts/sync-upstream.sh
