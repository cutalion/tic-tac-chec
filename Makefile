TUI_BINARY := tic-tac-chec
CLI_BINARY := tic-tac-chec-cli
SKILL_DIR := $(HOME)/.claude/skills/play-tic-tac-chec
SKILL_SRC := claude-skill/SKILL.md

.PHONY: build test install-skill install

build:
	go build -o $(TUI_BINARY) ./cmd/tui
	go build -o $(CLI_BINARY) ./cmd/cli

test:
	go test ./...

install-skill:
	go build -o $(CLI_BINARY) ./cmd/cli
	mkdir -p $(SKILL_DIR)
	cp $(CLI_BINARY) $(SKILL_DIR)/
	sed 's|__SKILL_DIR__|$(SKILL_DIR)|g' $(SKILL_SRC) > $(SKILL_DIR)/SKILL.md
	@echo "Skill installed to $(SKILL_DIR)"
	@echo "Restart Claude Code to pick it up."

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(TUI_BINARY) $(HOME)/.local/bin/
	cp $(CLI_BINARY) $(HOME)/.local/bin/
	@echo "Installed $(TUI_BINARY) and $(CLI_BINARY) to $(HOME)/.local/bin/"
