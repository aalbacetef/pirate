
tidy:
	go mod tidy 

fmt: tidy 
	goimports -w .

mk-bin-dir:
	mkdir -p bin/

build: fmt mk-bin-dir 
	go build -o bin/ ./cmd/pirate/ 

test: fmt
	go test -v ./...

.PHONY: tidy fmt mk-bin-dir build test
