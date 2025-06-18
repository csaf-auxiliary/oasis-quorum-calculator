# Project variables
APP_NAME := oqcd
BUILD_DIR := bin

default: build


# version calculation from git, see documentation in the code code comments of
#   https://github.com/gocsaf/csaf/blob/main/Makefile
GITDESC := $(shell git describe --tags --always --dirty=-modified --broken)
GITDESCPATCH := $(shell echo '$(GITDESC)' | sed -E 's/v?[0-9]+\.[0-9]+\.([0-9]+)[-+]?.*/\1/')
SEMVERPATCH := $(shell echo $$(( $(GITDESCPATCH) + 1 )))
SEMVER := $(shell echo '$(GITDESC)' | sed -E -e 's/^v//' -e 's/([0-9]+\.[0-9]+\.)([0-9]+)(-[1-9].*)/\1$(SEMVERPATCH)\3/' )
testsemver:
	@echo from \'$(GITDESC)\' transformed to \'$(SEMVER)\'

# Set -ldflags parameter to pass the semversion.
LDFLAGS = -ldflags "-X github.com/csaf-auxiliary/oasis-quorum-calculator/pkg/version.SemVersion=$(SEMVER)"

# Go commands
GO := go
GOFMT := go fmt
GOTEST := go test ./...
GOBUILD := go build -o $(BUILD_DIR)/$(APP_NAME) $(LDFLAGS) ./cmd/$(APP_NAME)

.PHONY: all build run test fmt clean

all: build

build:
	$(GOBUILD)
	go build $(LDFLAGS) -o $(BUILD_DIR)/sendaccountmails ./cmd/sendaccountmails
	go build $(LDFLAGS) -o $(BUILD_DIR)/createusers ./cmd/createusers
	go build $(LDFLAGS) -o $(BUILD_DIR)/importcommittee ./cmd/importcommittee
	go build $(LDFLAGS) -o $(BUILD_DIR)/exportmeeting ./cmd/exportmeeting

run: build
	./$(BUILD_DIR)/$(APP_NAME)

test:
	$(GOTEST)

fmt:
	$(GOFMT) ./...

clean:
	rm -rf $(BUILD_DIR)

