build:
	go build .
	mkdir -p build
	mv docker-registry-auth build/
.PHONY: build

run: build
	forego run -e .env build/docker-registry-auth
.PHONY: run
