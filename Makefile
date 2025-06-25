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
	@echo ""
	@echo "✅ qgh installed successfully!"
	@echo ""
	@echo "To enable the 'ctrl-d' shortcut to change directories, add this to your shell config:"
	@echo ""
	@echo "For Bash/Zsh (add to ~/.bashrc or ~/.zshrc):"
	@echo "qgh() {"
	@echo "    command qgh \"\$$@\""
	@echo "    if [[ -f /tmp/qgh_cd ]]; then"
	@echo "        cd \"\$$(<\"/tmp/qgh_cd\")\""
	@echo "        rm /tmp/qgh_cd"
	@echo "    fi"
	@echo "}"
	@echo ""
	@echo "For Fish (save to ~/.config/fish/functions/qgh.fish):"
	@echo "function qgh"
	@echo "    command qgh \$$argv"
	@echo "    if test -f /tmp/qgh_cd"
	@echo "        cd (cat /tmp/qgh_cd)"
	@echo "        rm /tmp/qgh_cd"
	@echo "    end"
	@echo "end"
	@echo ""
	@echo "Then reload your shell or run: source ~/.bashrc (or ~/.zshrc)"

clean:
	rm -rf $(BUILD_DIR)

