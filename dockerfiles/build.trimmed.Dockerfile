FROM golang:1.24-bookworm AS base 

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

ENTRYPOINT ["/app/pirate"]
