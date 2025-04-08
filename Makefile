
tidy:
	go mod tidy 

fmt: tidy 
	goimports -w .

mk-bin-dir:
	mkdir -p bin/

flags =
runner ?= podman

#################
# General tasks #
#################

build: fmt mk-bin-dir 
	go build $(flags) -o bin/ ./cmd/pirate/ 

test: fmt
	go test -cover -coverprofile "$(test_profile_name)" -v ./...

lint: fmt
	golangci-lint run ./...

test_profile_name = test.coverage.out
test_coverage_html = test.coverage.html

test-coverage: test 
	go tool cover -o "$(test_coverage_html)" -html "$(test_profile_name)"

test-cleanup:
	rm -f "$(test_profile_name)" 
	rm -f "$(test_coverage_html)" 

.PHONY: tidy fmt mk-bin-dir build test lint test-coverage

##############
# Deployment #
##############

TAG_NAME = aalbacetef/pirate
TAG_VERSION ?= latest

build-img:
	podman build -t $(TAG_NAME):$(TAG_VERSION) -f dockerfiles/build.Dockerfile .

build-trimmed-img:
	podman build -t $(TAG_NAME):$(TAG_VERSION)-trimmed -f dockerfiles/build.trimmed.Dockerfile .


release: 
	env CGO_ENABLED=0 go build -trimpath -ldflags='-w -s' ./cmd/pirate/
	gh release create $$(git describe --abbrev=0) --generate-notes pirate


#######################
# Integration testing #
#######################

build-testing-img:
	$(runner) build -t pirate:testing -f dockerfiles/testing.Dockerfile .

container_opts = 

run-integration-test: build-testing-img
	$(runner) run $(container_opts) --rm -it pirate:testing

.PHONY: build-testing-img run-integration-test
