# docker-registry-auth

An auth service for Nitrous private Docker registry that conforms to the [Docker
Registry token auth spec](https://github.com/docker/distribution/blob/master/docs/spec/auth/token.md).

The auth service connects to the nitrousd database to authenticate and authorize
Docker clients acting on behalf of Nitrous users:

1. Users are *authenticated* using their Nitrous username and a token in the
  `tokens` table with `type=docker_registry` (currently manually created).
2. Users are *authorized* using their Nitrous username and their role in the
   org. The org is determined by the repository the user is trying to access.
   E.g. when trying to `docker pull registry.tld:5000/nitrous-io/busybox`, the
   org (slug) is `nitrous-io`.

### Development

1. Run a Docker registry container: `docker run -d -p 5000:5000 --name registry.n2odev.com -v `pwd`/registry:/etc/docker/registry registry:2`
2. Create config file for auth service: `cp .env-example .env; open .env`
3. Run the auth service: `make run` (you'll need to install
[forego](https://github.com/ddollar/forego) first: `go get -u github.com/ddollar/forego`)

#### Connecting from Docker hosts

User workstations that are running locally in development as Docker containers
will connect to the private Docker registry and the auth service via the gateway
of the `docker0` Docker bridge network (see
[this](https://github.com/docker/docker/issues/1143)).

Hostbuilder will inject "host:<IP of docker0 gateway>" and
"registry.n2odev.com:<IP of docker0 gateway>" into the `/etc/hosts` of these
containers, so the private Docker registry and auth service will be available
via `http://registry.n2odev.com:5000`/`http://host:5000` and `http://host:5002`
respectively.

### Running tests

Run tests with `make run`. A `docker_registry_auth_test` Postgres database will
be created, which can be used in your tests for functional testing.

### Go dependency management

This project uses the Go 1.5 vendor experiment so you should
`export GO15VENDOREXPERIMENT=1` in your shell.

I'm using [Glide](https://github.com/Masterminds/glide) for dependency
management because I just cannot get [godep](https://github.com/tools/godep) to
work with `GO15VENDOREXPERIMENT=1` (it refuses to copy packages to `vendor/`).
