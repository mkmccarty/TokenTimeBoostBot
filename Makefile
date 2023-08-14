APP=TokenTimeBoostBot
EXECUTABLE=TokenTimeBoostBot

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

WINDOWS=$(EXECUTABLE)_windows_amd64.exe
LINUX=$(EXECUTABLE)_linux_amd64
DARWIN=$(EXECUTABLE)_darwin_amd64
PI=$(EXECUTABLE)_linux_arm6
PI64=$(EXECUTABLE)_linux_arm64

#VERSION=$(shell git describe --tags --always --long --dirty)
VERSION=$(shell git describe --tags --always --long --dirty)

all: fmt build

.PHONY: run
run: darwin
	./$(DARWIN)

.PHONY: doc
doc:
	godoc -http=:6060 -index

host:
	go build -v -o $(EXECUTABLE)_$(GOOS)_$(GOARCH) -ldflags="-s -w -X main.Version=$(VERSION)"  

.PHONY: build
build: host
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
