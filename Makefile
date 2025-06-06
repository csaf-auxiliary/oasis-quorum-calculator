# Project variables
APP_NAME := oqcd
BUILD_DIR := bin

# Go commands
GO := go
GOFMT := go fmt
GOTEST := go test ./...
GOBUILD := go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

.PHONY: all build run test fmt clean

all: build

build:
	$(GOBUILD)
	go build -o $(BUILD_DIR)/sendaccountmails ./cmd/sendaccountmails
	go build -o $(BUILD_DIR)/createusers ./cmd/createusers
	go build -o $(BUILD_DIR)/importcommittee ./cmd/importcommittee
	go build -o $(BUILD_DIR)/exportmeeting ./cmd/exportmeeting

run: build
	./$(BUILD_DIR)/$(APP_NAME)

test:
	$(GOTEST)

fmt:
	$(GOFMT) ./...

clean:
	rm -rf $(BUILD_DIR)

