
tidy:
	go mod tidy 

fmt: tidy 
	goimports -w .

mk-bin-dir:
	mkdir -p bin/

flags =
runner ?= podman

build: fmt mk-bin-dir 
	go build $(flags) -o bin/ ./cmd/pirate/ 


test: fmt
	go test -v ./...

lint: fmt
	golangci-lint run ./...

.PHONY: tidy fmt mk-bin-dir build test lint


build-testing-img:
	$(runner) build -t pirate:testing -f testing.Dockerfile .

container_opts = 

run-testing-img: build-testing-img
	$(runner) run $(container_opts) --rm -it pirate:testing


.PHONY: build-testing-img run-testing-img
