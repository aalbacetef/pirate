FROM golang:1.24-bookworm AS base 
LABEL org.opencontainers.image.source https://github.com/aalbacetef/pirate

WORKDIR /build 

COPY *.go . 
COPY go.mod .
COPY go.sum .
COPY ./cmd/ ./cmd/ 
COPY ./scheduler/ ./scheduler/

RUN go build -trimpath -ldflags='-w -s' ./cmd/pirate/

FROM debian:bookworm AS final 

WORKDIR /app 

COPY --from=base /build/pirate ./ 

RUN apt update && apt install -yq jq curl tar gzip 

ENTRYPOINT ["/app/pirate"]
