# Set GO111MODULE=on for consistency with GO < 1.13.x
export GO111MODULE := on

# cd into the GOPATH to workaround ./... not following symlinks
_allpackages = $(shell ( cd $(SRC) && \
    go list ./... 2>&1 1>&3 | \
    grep -v -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)) 1>&2 ) 3>&1 | \
    grep -v -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)))

# memoize allpackages, so that it's executed only once and only if used
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)

# Q (quiet) prefixes commands with 2, making them silent (default behavior).
# It will print commands if the V parameter is equal 1.
Q = $(if $V,,@)

# M Prints a nice colored arrow prefix on messages
M = $(shell printf "\033[95;1m▶\033[0m")

GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2> /dev/null || true)
GIT_REF = $(shell git describe --always --tags --dirty=-unknown 2>/dev/null)
PKG_NAME = github.com/LourdesMoran/go-test-utils

# Add | separated patterns of packages/directories to skip.
GO_FILES = $(shell find . -name "*.go" | grep -vE ".git|.env|vendor")

### Build Targets ###

.PHONY: all deps run fmt goimports lint vet check list test cover cover-stdout cover-html install clean help

setup:
	go get -u github.com/wadey/gocovmerge
	go get -u golang.org/x/lint/golint

deps: ## Download the Go modules required by this project.
	$Q go mod download

fmt: ; $(info $(M) running gofmt…) @ ## Run gofmt
	$Q go fmt $(if $V, -x) ./...

lint: ; $(info $(M) running golint…) @ ## Run golint
	$Q command -v golint > /dev/null || go get $(if $V,-v) golang.org/x/lint/golint
	$Q golint ./...

vet: ; $(info $(M) running vet…) @ ## Run vet
	$Q go vet $(if $V, -v) ./...

check: fmt lint vet test ## Check Go code quality. Runs fmt, golint and vet.

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

cover: $(GO_FILES) ## Generates test coverage for this project
	$Q command -v gocovmerge > /dev/null || go get $(if $V,-v) github.com/wadey/gocovmerge
	$Q mkdir -p ./cover/
	$Q rm -f ./cover/*.out ./cover/all.merged
	$Q go test -coverprofile=./cover/coverage.out ./...

cover-stdout: cover
	$Q go tool cover -func ./cover/coverage.out

cover-html: cover
	$Q go tool cover -html ./cover/coverage.out


help: ## This help
	@echo "✨ Test Utils System ✨"
	@echo "Usage:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
