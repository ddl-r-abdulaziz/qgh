GO_SOURCES := $(wildcard *.go)
BUILD_DIR := build
TARGET := $(BUILD_DIR)/qgh

.PHONY: all clean install

all: $(TARGET)

$(TARGET): go.mod $(GO_SOURCES) | $(BUILD_DIR)
	go build -o $@ .

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

install: $(TARGET)
	sudo install $(TARGET) /usr/local/bin/

clean:
	rm -rf $(BUILD_DIR)

