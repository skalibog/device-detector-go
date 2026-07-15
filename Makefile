.PHONY: build test test-fixtures vet sync-upstream

build:
	go build ./...

test:
	go test ./...

test-fixtures:
	go test -run TestFixtures -v .

vet:
	go vet ./...

sync-upstream:
	bash scripts/sync-upstream.sh
