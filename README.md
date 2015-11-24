# docker-registry-auth

An auth service for Nitrous private Docker registry that conforms to the [Docker
Registry token auth spec](https://github.com/docker/distribution/blob/master/docs/spec/auth/token.md).

1. Run a Docker registry container: `docker run -d -p 5000:5000 --name ncr -v `pwd`/registry:/etc/docker/registry registry:2`
