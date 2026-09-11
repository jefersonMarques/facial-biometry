.PHONY: test build-web test-go test-engine

test: test-go test-engine

build-web:
	cd apps/web-sdk && npm install && npm run build

test-go:
	cd services/api && go test ./...

test-engine:
	cd services/engine && python -m unittest discover -s tests -v
