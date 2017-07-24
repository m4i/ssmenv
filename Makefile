OSARCH := linux/amd64 linux/386 darwin/amd64
STACK_NAME := SsmenvTest

NAME := $(notdir $(CURDIR))
OWNER := $(notdir $(shell dirname $(CURDIR)))
VERSION = $(shell git describe)
HEAD_TAG = $(shell git describe --exact-match HEAD)
PACKAGES = $(shell go list ./... | grep -v /vendor/)
COMMA_SEPARATED_PACKAGES = $(shell echo $(PACKAGES) | tr ' ' ,)

LDFLAGS = -ldflags '-s -w -X main.version=$(VERSION) -extldflags -static'
STATIC := -a -tags netgo -installsuffix netgo

.DEFAULT_GOAL := build

.PHONY: build
build:
	go build $(LDFLAGS) -o bin/$(NAME)

.PHONY: install
install:
	go install $(LDFLAGS)

.PHONY: deps
deps:
	go get github.com/golang/dep/cmd/dep
	dep ensure -v

.PHONY: clean
clean:
	rm -rf bin

.PHONY: test
test: test-go lint

.PHONY: test-go
test-go:
	go test -race -v -coverprofile coverage.txt -covermode atomic -coverpkg $(COMMA_SEPARATED_PACKAGES)

.PHONY: lint
lint:
ifeq ($(ENABLE),)
	go get github.com/alecthomas/gometalinter
	gometalinter --install
	gometalinter --config gometalinter.json ./...
else
	gometalinter --config gometalinter.json --disable-all --enable $(ENABLE) ./...
endif

.PHONY: cross-build
cross-build:
	set -eu; \
	for osarch in $(OSARCH); do \
		os=$${osarch%/*} arch=$${osarch#*/}; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
		go build $(LDFLAGS) $(STATIC) -o bin/$(NAME)_$${os}_$$arch; \
	done

.PHONY: cross-build-docker
cross-build-docker:
	docker run --rm \
		--volume $(CURDIR):/go/src/github.com/$(OWNER)/$(NAME) \
		--workdir /go/src/github.com/$(OWNER)/$(NAME) \
		golang:1 \
		sh -c 'make deps && make cross-build'

.PHONY: tag
tag:
ifeq ($(TAG),)
	@echo 'Usage: make tag TAG=vX.Y.Z' >&2
else
	git tag --annotate --message $(TAG) $(TAG)
endif

.PHONY: release
release:
	set -eu; \
	tag=$(HEAD_TAG); \
	if [ -n "$$tag" ]; then \
		if [ -z "$$(git ls-remote origin $$tag)" ]; then \
			git push origin $$tag; \
		fi; \
		go get github.com/tcnksm/ghr; \
		ghr --username $(OWNER) $$tag bin; \
	fi

.PHONY: test-stack
test-stack:
ifeq ($(SSMENV_TEST_REGION),)
	@echo '$$SSMENV_TEST_REGION is required' >&2
else
	set -eu; \
	describe() { \
		aws cloudformation describe-stacks \
			--region $(SSMENV_TEST_REGION) \
			--stack-name $(STACK_NAME) \
			--query 'Stacks[0].Outputs' \
			--output text; \
	}; \
	\
	crate_or_update=create; \
	if describe >/dev/null 2>&1; then crate_or_update=update; fi; \
	\
	aws cloudformation $$crate_or_update-stack \
		--region $(SSMENV_TEST_REGION) \
		--stack-name $(STACK_NAME) \
		--template-body file://test-cloudformation.yml \
		--capabilities CAPABILITY_IAM \
		--output text; \
	aws cloudformation wait stack-$$crate_or_update-complete \
		--region $(SSMENV_TEST_REGION) \
		--stack-name $(STACK_NAME); \
	describe
endif

.PHONY: delete-test-stack
delete-test-stack:
ifeq ($(SSMENV_TEST_REGION),)
	@echo '$$SSMENV_TEST_REGION is required' >&2
else
	aws cloudformation delete-stack \
		--region $(SSMENV_TEST_REGION) \
		--stack-name $(STACK_NAME)
endif
