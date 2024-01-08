# Change these variables as necessary.

UNAME_A = `uname -m`
UNAME_S = `uname -s`

#// Get machine architecture into ARCH variable
#ifeq ($(UNAME_A),aarch64)
#	ARCH = arm64
#else ifeq ($(UNAME_A),armv7l)
#	ARCH = arm

APP=TokenTimeBoostBot
BINARY_NAME=TokenTimeBoostBot

BUILD_OUTPUT=build

WINDOWS=$(BINARY_NAME)_windows_amd64.exe
LINUX=$(BINARY_NAME)_linux_amd64
DARWIN=$(BINARY_NAME)_darwin_amd64
PI=$(BINARY_NAME)_linux_arm6
PI64=$(BINARY_NAME)_linux_arm64
BSD=$(BINARY_NAME)_freebsd_amd64

#VERSION=$(shell git describe --tags --always --long --dirty)
VERSION=$(shell git describe --tags --always --long --dirty)

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code



# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #


## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go test -race -buildvcs -vet=off ./...

.PHONY: lint
lint:
	golangci-lint run --out-format=github-actions -v --new-from-rev HEAD~5

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

## build: build the application

all: fmt build

.PHONY: run
run: darwin
	./$(DARWIN)

.PHONY: doc
doc:
	godoc -http=:6060 -index

windows: $(WINDOWS) ## Build for Windows

linux: $(LINUX) ## Build for Linux

darwin: $(DARWIN)  ## Build for Darwin

pi: $(PI) ## Build for Raspberry Pi 4

pi64: $(PI64) ## Build for 64-bit Raspberry Pi

freebsd: $(BSD) ## Build for FreeBSD


$(WINDOWS):
	env GOOS=windows GOARCH=amd64 go build -v -o $(BUILD_OUTPUT)/$(WINDOWS) -ldflags="-s -w -X main.Version=$(VERSION)"

$(LINUX):
	env GOOS=linux GOARCH=amd64 go build -v -o $(BUILD_OUTPUT)/$(LINUX) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(DARWIN):
	env GOOS=darwin GOARCH=amd64 go build -v -o $(BUILD_OUTPUT)/$(DARWIN) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(PI):
	env GOOS=linux GOARCH=arm GOARM=6  go build -v -o $(BUILD_OUTPUT)/$(PI) -ldflags="-s -w -X main.Version=$(VERSION)"  

$(PI64):
	env GOOS=linux GOARCH=arm64  go build -v -o $(BUILD_OUTPUT)/$(PI64) -ldflags="-s -w -X main.Version=$(VERSION)"  

$(BSD):
	env GOOS=freebsd GOARCH=amd64  go build -v -o $(BUILD_OUTPUT)/$(BSD) -ldflags="-s -w -X main.Version=$(VERSION)"  

.PHONY: build
build: windows linux darwin pi pi64 freebsd ## Build binaries
	@echo version: $(VERSION)


.PHONY: install
install:
	./scripts/stop_bot.sh
	cp $(BUILD_OUTPUT)/$(PI64) ~/bots/TokenTimeBoostBot
	./scripts/start_bot.sh

.PHONY: clean
clean:
	go clean
	rm -r $(BUILD_OUTPUT)/*

