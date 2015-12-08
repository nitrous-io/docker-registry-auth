build: bootstrap
	gb build all
.PHONY: build

run: build
	forego run -e .env bin/docker-registry-auth
.PHONY: run

test: bootstrap create-test-db
	gb test all

create-test-db:
	createdb -E UTF8 -T template0 -l en_US.utf8 docker_registry_auth_test || true
.PHONY: create-test-db

drop-test-db:
	dropdb --if-exists docker_registry_auth_test
.PHONY: drop-test-db

bootstrap:
	@test -x $(GOPATH)/bin/gb || ( \
		go get github.com/constabulary/gb/... && \
		go install github.com/constabulary/gb \
	)
.PHONY: bootstrap
