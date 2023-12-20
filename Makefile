APP=TokenTimeBoostBot
BINARY_NAME=TokenTimeBoostBot
WINDOWS=$(BINARY_NAME)_windows_amd64.exe
LINUX=$(BINARY_NAME)_linux_amd64
DARWIN=$(BINARY_NAME)_darwin_amd64
PI=$(BINARY_NAME)_linux_arm6
#PI64=$(EXECUTABLE)_linux_arm64

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



#VERSION=$(shell git describe --tags --always --long --dirty)
VERSION=$(shell git describe --tags --always --long --dirty)

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

#pi64: $(PI64) ## Build for 64-bit Raspberry Pi


$(WINDOWS):
	env GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS) -ldflags="-s -w -X main.Version=$(VERSION)"

$(LINUX):
	env GOOS=linux GOARCH=amd64 go build -v -o $(LINUX) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(DARWIN):
	env GOOS=darwin GOARCH=amd64 go build -v -o $(DARWIN) -ldflags="-s -w -X main.Version=$(VERSION)" 

$(PI):
	env GOOS=linux GOARCH=arm GOARM=6  go build -v -o $(PI) -ldflags="-s -w -X main.Version=$(VERSION)"  

#$(PI64):
#	env GOOS=linux GOARCH=arm64  go build -v -o $(PI) -ldflags="-s -w -X main.Version=$(VERSION)"  

.PHONY: build
build: windows linux darwin pi ## Build binaries
	@echo version: $(VERSION)


.PHONY: test
test:
	go test -timeout 20s -race #-v ./...

.PHONY: debs
debs:
	go get ./...

.PHONY: fmt
fmt:
	gofmt -l -s .

.PHONY: install
install:
	./scripts/stop_bot.sh
	cp $(PI) ~pi/bots/TokenTimeBoostBot
	./scripts/start_bot.sh

.PHONY: clean
clean:
	go clean
	rm -f $(WINDOWS) $(LINUX) $(DARWIN) $(PI)

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
