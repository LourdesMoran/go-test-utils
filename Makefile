# Set GO111MODULE=on for consistency with GO < 1.13.x
export GO111MODULE := on

### Parameters ###

# When DEBUG is set, set log level to debug.
# DEBUG := 0
# When V (verbose) is set, print commands and build progress.
# V := 0

# Name of the config file to use for running skyway.
CONFIG := config.local

### Environment Variables ###

# Q (quiet) prefixes commands with 2, making them silent (default behavior).
# It will print commands if the V parameter is equal 1.
Q = $(if $V,,@)

# M Prints a nice colored arrow prefix on messages
M = $(shell printf "\033[95;1m▶\033[0m")

GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2> /dev/null || true)
GIT_REF = $(shell git describe --always --tags --dirty=-unknown 2>/dev/null)
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2> /dev/null)
PKG_NAME = github.com/GooeeIOT/firmware-skyway


# Get the Version from the `VERSION` file.
ifneq (,$(wildcard VERSION))
V_FILE = $(strip $(shell cat VERSION 2> /dev/null))
endif

ifeq ($(GIT_BRANCH),master)
	VERSION = $(GIT_COMMIT)-dev
	TAGS += dev
else ifeq ($(GIT_BRANCH),prod)
	VERSION = $(V_FILE)
	TAGS += prod
else ifeq ($(filter rel-%,$(GIT_BRANCH)),$(GIT_BRANCH))
	VERSION = $(V_FILE)
	TAGS += release
else
	VERSION = $(GIT_REF)
	TAGS += other
endif

# Add | separated patterns of packages/directories to skip.
GO_FILES = $(shell find . -name "*.go" | grep -vE ".git|.env|vendor|release")


### Build Targets ###

.PHONY: all deps run fmt goimports lint vet check list test cover cover-junit cover-stdout cover-html release install clean help

all: install

deps: ## Download the Go modules required by this project.
	$Q go mod download

run:  ## Run the skyway program for testing, using debug mode (Requires config file to be present)
	$Q go run $(if ${DEBUG}, -race) $(if $V, -v) ./cmd/skyway/main.go --config $(CONFIG) $(if ${DEBUG}, --gooee.logging.level debug)

fmt: ; $(info $(M) running gofmt…) @ ## Run gofmt
	$Q go fmt $(if $V, -x) ./...

goimports: ; $(info $(M) running goimports…) @ ## Run goimports
	$Q command -v goimports > /dev/null || go get $(if $V,-v) golang.org/x/tools/cmd/goimports
	$Q find ./ -iname \*.go | grep -v vendor | xargs goimports -w -l $(if $V,-v)

lint: ; $(info $(M) running golint…) @ ## Run golint
	$Q command -v golint > /dev/null || go get $(if $V,-v) golang.org/x/lint/golint
	$Q golint ./..

vet: ; $(info $(M) running vet…) @ ## Run vet
	$Q go vet $(if $V, -v) ./...

check: fmt goimports lint vet ## Check Go code quality. Runs fmt, golint, goimports and vet.

list: ## List Go packages used by this project
	$Q go list ./...


TEST_TARGETS := test-default test-bench test-verbose test-race
.PHONY: $(TEST_TARGETS) test tests
test-bench:   ARGS=-run=__nothing__ -bench=. 		        ## Run benchmarks
test-verbose: ARGS=-v            							## Run tests in verbose mode
test-race:    ARGS=-race         							## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
test tests: vet; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q go test $(ARGS) ./...

ci:
	$Q golangci-lint run

### TODO: Add -race to go test and -set-exit-code to go-junit-report here once
###       we fix some race conditions happening currently.
cover-junit: $(GO_FILES)
	$Q command -v go-junit-report > /dev/null || go get $(if $V,-v) github.com/jstemmer/go-junit-report
	$Q mkdir -p cover
	$Q rm -f cover/coverage.xml cover/coverage.log
	$Q go clean -testcache ./...
	$Q go test -v ./... >&1 | tee cover/coverage.log
	$Q cat cover/coverage.log | go-junit-report > cover/coverage.xml

cover: $(GO_FILES) ## Generates test coverage for this project
	$Q command -v gocovmerge > /dev/null || go get $(if $V,-v) github.com/wadey/gocovmerge
	$Q mkdir -p ./cover/
	$Q rm -f ./cover/*.out ./cover/all.merged
	$Q go test -coverprofile=./cover/coverage.out ./...

cover-stdout: cover
	$Q go tool cover -func ./cover/coverage.out


cover-html: cover
	$Q go tool cover -html ./cover/coverage.out


RELEASE_DIR ?= release
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOEXE = $(shell GOOS=$(GOOS) GOARCH=$(GOARCH) go env GOEXE)
# CGO is required for compiling "zmq4".
CGO_ENABLED ?= 1

GOVARS += -X $(PKG_NAME)/pkg/common/version.version=${VERSION}
GOVARS += -X $(PKG_NAME)/pkg/common/version.gitBranch=${GIT_BRANCH}
GOVARS += -X $(PKG_NAME)/pkg/common/version.gitCommit=${GIT_COMMIT}
GOVARS += -X $(PKG_NAME)/pkg/common/version.buildDate=${BUILD_DATE}
LDFLAGS = -ldflags "-w $(GOVARS)"
GOBUILD = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build ${LDFLAGS} -tags "${TAGS}" -o "$@"

skyway: $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/skyway$(GOEXE)

$(RELEASE_DIR)/$(GOOS)-$(GOARCH)/skyway$(GOEXE): $(GO_FILES)
	$Q $(GOBUILD) ./cmd/skyway
	$Q echo "version: ${VERSION}" >> $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/INFO
	$Q echo "buildDate: ${BUILD_DATE}" >> $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/INFO
	$Q echo "gitBranch: ${GIT_BRANCH}" >> $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/INFO
	$Q echo "gitCommit: ${GIT_COMMIT}" >> $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/INFO
	$Q cp -rf configs/config.local.yaml $(RELEASE_DIR)/$(GOOS)-$(GOARCH)/
	$Q tar -zcf $(RELEASE_DIR)/skyway-$(GOOS)-$(GOARCH).tar.gz $(RELEASE_DIR)/$(GOOS)-$(GOARCH)
	$Q sha256sum $(RELEASE_DIR)/skyway-$(GOOS)-$(GOARCH).tar.gz > $(RELEASE_DIR)/skyway-$(GOOS)-$(GOARCH).sha256

release: skyway ## Releases a new Skyway version.

clean_release: ## Cleans artifact from the current GOOS and GOARCH release.
	$Q rm -rf $(RELEASE_DIR)/$(GOOS)-$(GOARCH)*

install: deps ## Install binaries locally for testing
	$Q go install $(if $V,-v) ./cmd/skyway
	$Q go install $(if $V,-v) ./cmd/dal-mock

clean: ## Clean up all artifacts
	[ -d $(RELEASE_DIR) ] && rm -rf $(RELEASE_DIR) || [ ! -d $(RELEASE_DIR) ]
	[ -d cover/ ] && rm -rf cover/ || [ ! -d cover/ ]

help: ## This help
	@echo "✨ Skyway Build System ✨"
	@echo "Usage:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
