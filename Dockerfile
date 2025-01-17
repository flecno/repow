FROM golang:1.15-buster AS build

ARG VERSION=undefinied

WORKDIR /src/
COPY cmd/ /src/cmd/
COPY internal/ /src/internal/
COPY go.mod /src/
COPY go.sum /src/
COPY Makefile /src/
WORKDIR /src/
ENV GO111MODULE=on
RUN make build-linux64

FROM debian:10.9
RUN apt-get update
RUN apt-get install -y ca-certificates
COPY --from=build /src/bin/repow_linux-amd64 /bin/repow
EXPOSE 8080/tcp
ENTRYPOINT ["/bin/repow"]
CMD ["serve"]
