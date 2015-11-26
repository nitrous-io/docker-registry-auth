build:
	go build .
	mkdir -p build
	mv docker-registry-auth build/
.PHONY: build

run: build
	forego run -e .env build/docker-registry-auth
.PHONY: run

test: create-test-db
	go test .

create-test-db:
	createdb -E UTF8 -T template0 -l en_US.utf8 docker_registry_auth_test || true
.PHONY: create-test-db

drop-test-db:
	dropdb --if-exists docker_registry_auth_test
.PHONY: drop-test-db
