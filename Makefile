# sei-internal-skills repo — workspace setup targets.
#
# Targets are read-only by design: they install agents and read-only permission
# patterns into a user's Claude workspace. Mutating wrappers (close-issue,
# merge-pr, apply-flux) are explicitly out of scope — those go through normal
# git/PR flow.

.DEFAULT_GOAL := help

SHELL := /usr/bin/env bash

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-32s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: update
update: ## ⭐ Get current: fast-forward this checkout, then sync ALL skills/agents/output-styles into ~/.claude + verify
	@echo "→ fast-forwarding this sei-internal-skills checkout (run on main)…"
	@git pull --ff-only
	@$(MAKE) --no-print-directory sync-all

.PHONY: sync-all
sync-all: ## Sync ALL skills+agents (portable+sei) + output styles into ~/.claude + verify — no git pull (used by `update` and the over-the-wire installer)
	@echo "→ syncing all agents…"
	@./scripts/sync-agents.sh --target ~/ --categories all --force
	@echo "→ syncing all skills…"
	@./scripts/sync-skills.sh --target ~/ --categories all --force
	@$(MAKE) --no-print-directory verify-catalog
	@echo "→ syncing output styles…"
	@./scripts/sync-output-styles.sh --target ~/ --force
	@./scripts/prune-retired.sh --target ~/ --check
	@echo "✓ environment current with sei-internal-skills $$(git rev-parse --short HEAD)"

.PHONY: verify-catalog
verify-catalog: ## Fail if any skill/agent declares a category that maps to no sync alias (orphaned-skill guard; CI)
	@./scripts/sync-skills.sh --verify
	@./scripts/sync-agents.sh --verify

.PHONY: bootstrap
bootstrap: sync-agents sync-skills sync-output-styles update-agent-permissions ## Install PORTABLE agents+skills+output-styles+permissions into a consumer env (external repos). For your own env use `make update`.

.PHONY: sync-agents
sync-agents: ## Install sei-internal-skills's portable agents into ~/.claude/agents/
	@./scripts/sync-agents.sh --target ~/ --categories portable --force

.PHONY: sync-skills
sync-skills: ## Install sei-internal-skills's portable skills into ~/.claude/skills/
	@./scripts/sync-skills.sh --target ~/ --categories portable --force

.PHONY: sync-output-styles
sync-output-styles: ## Install sei-internal-skills's output styles into ~/.claude/output-styles/ (ships the file; activation stays opt-in)
	@./scripts/sync-output-styles.sh --target ~/ --force

.PHONY: sync-experimental
sync-experimental: ## OPT-IN: install experimental/ skills+agents into ~/.claude. Never runs as part of update/sync-all/bootstrap.
	@./scripts/sync-experimental.sh --target ~/ --force

.PHONY: prune-retired
prune-retired: ## Report which retired/parked resources are still installed in ~/.claude. Read-only — deletes nothing.
	@./scripts/prune-retired.sh --target ~/

.PHONY: prune-retired-apply
prune-retired-apply: ## DELETES the retired + parked resources listed by `make prune-retired` from ~/.claude. The only target here that removes anything.
	@./scripts/prune-retired.sh --target ~/ --apply

.PHONY: sync-doctrine-self
sync-doctrine-self: ## Re-inject the operating-doctrine block into this repo's own AGENTS.md (dogfood; run after editing scripts/sei-internal-skills-doctrine.md)
	@bash -c '. ./scripts/lib/inject-doctrine.sh && inject_doctrine "." "./scripts/sei-internal-skills-doctrine.md" write'

.PHONY: sync-doctrine-self-check
sync-doctrine-self-check: ## Fail if this repo's AGENTS.md doctrine block has drifted from scripts/sei-internal-skills-doctrine.md (read-only; CI guard)
	@bash -c '. ./scripts/lib/inject-doctrine.sh && inject_doctrine "." "./scripts/sei-internal-skills-doctrine.md" check' \
		&& echo "doctrine block in sync ✓"

.PHONY: test-doctrine
test-doctrine: ## Run the doctrine-injector regression suite (scripts/tests/inject-doctrine.test.sh)
	@./scripts/tests/inject-doctrine.test.sh

.PHONY: test-output-styles
test-output-styles: ## Run the output-style syncer regression suite (scripts/tests/sync-output-styles.test.sh)
	@./scripts/tests/sync-output-styles.test.sh

.PHONY: test-experimental
test-experimental: ## Run the experimental-tier isolation suite (nothing in experimental/ ships by default)
	@./scripts/tests/experimental-isolation.test.sh

.PHONY: test-install
test-install: ## Run the installer regression suite — targeted mode (scripts/tests/install.test.sh)
	@./scripts/tests/install.test.sh

.PHONY: test-prune
test-prune: ## Run the prune-retired regression suite (never deletes core or user-authored resources)
	@./scripts/tests/prune-retired.test.sh

.PHONY: update-agent-permissions
update-agent-permissions: ## Install canonical read-only allow-list into ./.claude/settings.json (DRY_RUN=1 to preview)
	@./scripts/update-agent-permissions.sh

.PHONY: verify-agent-permissions
verify-agent-permissions: ## Fail if .claude/settings.json contains mutating patterns or has drifted
	@./scripts/verify-agent-permissions.sh

# --- sei-agent-driver (Go) ---------------------------------------------------
#
# The one Go module in this repo. Kept behind its own targets rather than folded
# into the workspace ones above, because those install into a user's ~/.claude and
# this builds a binary — a contributor working on skills should never need a Go
# toolchain to run them.

.PHONY: driver-check
driver-check: ## Everything enforced about sei-agent-driver: fmt, build, vet, test, tidy (CI)
	@cd sei-agent-driver && \
	  unformatted="$$(gofmt -l .)"; \
	  if [ -n "$$unformatted" ]; then \
	    printf 'gofmt: these files need formatting:\n%s\n' "$$unformatted" >&2; exit 1; \
	  fi
	cd sei-agent-driver && go build ./...
	cd sei-agent-driver && go vet ./...
	cd sei-agent-driver && go test ./... -race
	cd sei-agent-driver && go mod tidy -diff

# -buildvcs=false because a nested module cannot stamp VCS info, which is also
# why the version is passed in: without it the binary cannot say which commit it
# is, and neither can runtime/debug.ReadBuildInfo.
DRIVER_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: driver-build
driver-build: ## Build the sei-agent-driver binary into sei-agent-driver/bin/
	cd sei-agent-driver && go build -trimpath -buildvcs=false \
	  -ldflags "-X main.version=$(DRIVER_VERSION)" \
	  -o bin/sei-agent-driver ./cmd/sei-agent-driver
