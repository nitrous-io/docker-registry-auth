FROM golang:1.5.1

COPY . /go/src/github.com/nitrous-io/docker-registry-auth
WORKDIR /go/src/github.com/nitrous-io/docker-registry-auth

ENV GO15VENDOREXPERIMENT 1

RUN go get github.com/ddollar/forego
RUN make build

CMD ["forego", "run", "-e", ".env", "build/docker-registry-auth"]
