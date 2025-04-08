FROM golang:1.24-bookworm AS builder

WORKDIR /app 

RUN go install golang.org/x/tools/cmd/goimports@latest
RUN apt update && apt install make bash curl

COPY . . 

ENV CGO_ENABLED=0
RUN make build flags='-trimpath -ldflags="-w -s"'

CMD "/app/tasks/integration.test.sh"
