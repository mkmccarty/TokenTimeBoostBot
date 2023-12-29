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

WINDOWS=$(BINARY_NAME)_windows_amd64.exe
LINUX=$(BINARY_NAME)_linux_amd64
DARWIN=$(BINARY_NAME)_darwin_amd64
PI=$(BINARY_NAME)_linux_arm6
#PI64=$(EXECUTABLE)_linux_arm64

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

i64: $(PI64) ## Build for 64-bit Raspberry Pi


$(WINDOWS):
	env GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS) -ldflags="-s -w -X main.Version=$(VERSION)"

$(LINUX):
	env GOOS=linux GOARCH=amd64 go build -v -o $(LINUX) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(DARWIN):
	env GOOS=darwin GOARCH=amd64 go build -v -o $(DARWIN) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(PI):
	env GOOS=linux GOARCH=arm GOARM=6  go build -v -o $(PI) -ldflags="-s -w -X main.Version=$(VERSION)"  

$(PI64):
	env GOOS=linux GOARCH=arm64  go build -v -o $(PI64) -ldflags="-s -w -X main.Version=$(VERSION)"  

.PHONY: build
build: windows linux darwin pi ## Build binaries
	@echo version: $(VERSION)


.PHONY: install
install:
	./scripts/stop_bot.sh
	cp $(PI) ~pi/bots/TokenTimeBoostBot
	./scripts/start_bot.sh

.PHONY: clean
clean:
	go clean
	rm -f $(WINDOWS) $(LINUX) $(DARWIN) $(PI) $(PI5)

