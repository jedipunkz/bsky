BINARY_NAME := bsky
INSTALL_DIR := $(HOME)/.bin

.PHONY: build install lint

build:
	go build -o $(BINARY_NAME) .

install: build
	mkdir -p $(INSTALL_DIR)
	mv $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

lint:
	golangci-lint run
