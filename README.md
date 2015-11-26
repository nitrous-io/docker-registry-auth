# docker-registry-auth

An auth service for Nitrous private Docker registry that conforms to the [Docker
Registry token auth spec](https://github.com/docker/distribution/blob/master/docs/spec/auth/token.md).

The auth service connects to the nitrousd database to authenticate and authorize
Docker clients acting on behalf of nitrous users.

1. Run a Docker registry container: `docker run -d -p 5000:5000 --name ncr -v `pwd`/registry:/etc/docker/registry registry:2`
2. Create config file for auth service: `cp .env-example .env; open .env`
3. Run the auth service: `make run` (you'll need to install
[forego](https://github.com/ddollar/forego) first: `go get -u github.com/ddollar/forego`)

### Running tests

Run tests with `make run`. A `docker_registry_auth_test` database will be
created, which can be used in your tests for functional testing.

### Go dependency management

This project uses the Go 1.5 vendor experiment so you should
`export GO15VENDOREXPERIMENT=1` in your shell.

I'm using [Glide](https://github.com/Masterminds/glide) for dependency
management because I just cannot get [godep](https://github.com/tools/godep) to
work with `GO15VENDOREXPERIMENT=1` (it refuses to copy packages to `vendor/`).
