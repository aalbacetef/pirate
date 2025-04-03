FROM golang:1.24-bookworm AS builder

WORKDIR /app 

RUN go install golang.org/x/tools/cmd/goimports@latest
RUN apt update && apt install make bash curl

COPY . . 

RUN make build

CMD "/app/tasks/integration.test.sh"
