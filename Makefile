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
update: ## ⭐ Get current: fast-forward this checkout, then sync ALL skills/agents into ~/.claude + verify
	@echo "→ fast-forwarding this sei-internal-skills checkout (run on main)…"
	@git pull --ff-only
	@$(MAKE) --no-print-directory sync-all

.PHONY: sync-all
sync-all: ## Sync ALL skills+agents (portable+sei) into ~/.claude + verify — no git pull (used by `update` and the over-the-wire installer)
	@echo "→ syncing all agents…"
	@./scripts/sync-agents.sh --target ~/ --categories all --force
	@echo "→ syncing all skills…"
	@./scripts/sync-skills.sh --target ~/ --categories all --force
	@$(MAKE) --no-print-directory verify-catalog
	@echo "✓ environment current with sei-internal-skills $$(git rev-parse --short HEAD)"

.PHONY: verify-catalog
verify-catalog: ## Fail if any skill/agent declares a category that maps to no sync alias (orphaned-skill guard; CI)
	@./scripts/sync-skills.sh --verify
	@./scripts/sync-agents.sh --verify

.PHONY: bootstrap
bootstrap: sync-agents sync-skills update-agent-permissions ## Install PORTABLE agents+skills+permissions into a consumer env (external repos). For your own env use `make update`.

.PHONY: sync-agents
sync-agents: ## Install sei-internal-skills's portable agents into ~/.claude/agents/
	@./scripts/sync-agents.sh --target ~/ --categories portable --force

.PHONY: sync-skills
sync-skills: ## Install sei-internal-skills's portable skills into ~/.claude/skills/
	@./scripts/sync-skills.sh --target ~/ --categories portable --force

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

.PHONY: update-agent-permissions
update-agent-permissions: ## Install canonical read-only allow-list into ./.claude/settings.json (DRY_RUN=1 to preview)
	@./scripts/update-agent-permissions.sh

.PHONY: verify-agent-permissions
verify-agent-permissions: ## Fail if .claude/settings.json contains mutating patterns or has drifted
	@./scripts/verify-agent-permissions.sh
