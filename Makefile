.PHONY: build clean run test

BINARY_NAME=legacycoin-miner
BUILD_DIR=build

all: build

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go

clean:
	rm -rf $(BUILD_DIR)

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) -wallet=YOUR_WALLET_ADDRESS

test:
	go test -v ./...

install:
	go install

deps:
	go mod init github.com/yourusername/legacycoin-miner
	go get github.com/gorilla/websocket
	go get github.com/go-sql-driver/mysql
