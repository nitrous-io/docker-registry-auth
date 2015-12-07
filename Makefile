build: bootstrap
	GO15VENDOREXPERIMENT=1 go build .
	mkdir -p build
	mv docker-registry-auth build/
.PHONY: build

run: build
	forego run -e .env build/docker-registry-auth
.PHONY: run

test: bootstrap create-test-db
	GO15VENDOREXPERIMENT=1 go test $(shell glide novendor)

create-test-db:
	createdb -E UTF8 -T template0 -l en_US.utf8 docker_registry_auth_test || true
.PHONY: create-test-db

drop-test-db:
	dropdb --if-exists docker_registry_auth_test
.PHONY: drop-test-db

bootstrap:
	@test -x /usr/local/bin/glide || ( \
		go get github.com/Masterminds/glide && \
		rm -rf $(GOPATH)/src/github.com/Masterminds/glide/vendor && \
		cd $(GOPATH)/src/github.com/Masterminds/glide && \
		GO15VENDOREXPERIMENT=1 make bootstrap && \
		GO15VENDOREXPERIMENT=1 make build && \
		sudo cp glide /usr/local/bin/glide && \
		cd - && \
		glide install \
	)
.PHONY: bootstrap
