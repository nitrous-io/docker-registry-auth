FROM nitrousio/nitrous-ubuntu-dind:14.04-1.7.0-nitrous1
MAINTAINER Nitrous.IO <eng@nitrous.io>

RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  curl \
  postgresql-9.3 \
  postgresql-contrib-9.3

RUN locale-gen en_US.utf8

ENV PATH /usr/local/go/bin:/opt/bin:$PATH
ENV GOBIN /opt/bin
ENV GOPATH /opt

# install go
RUN curl -s https://storage.googleapis.com/golang/go1.5.2.linux-amd64.tar.gz \
  | tar -C /usr/local -xz

RUN \
  go get github.com/ddollar/forego && \
  go install github.com/ddollar/forego && \
  go get github.com/constabulary/gb && \
  go install github.com/constabulary/gb/...

# postgres settings for testing
RUN \
 /etc/init.d/postgresql start && \
  su postgres -c "psql -c 'CREATE ROLE root SUPERUSER CREATEDB CREATEROLE INHERIT LOGIN;'" && \
  echo "fsync=off" >> /etc/postgresql/9.3/main/postgresql.conf && \
  echo "synchronous_commit=off" >> /etc/postgresql/9.3/main/postgresql.conf && \
  echo "full_page_writes=off" >> /etc/postgresql/9.3/main/postgresql.conf

ADD . /opt/src/github.com/nitrous-io/docker-registry-auth
WORKDIR /opt/src/github.com/nitrous-io/docker-registry-auth
