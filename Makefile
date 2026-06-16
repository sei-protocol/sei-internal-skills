# Tide repo — workspace setup targets.
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

.PHONY: bootstrap
bootstrap: sync-agents sync-skills update-agent-permissions ## One-shot: install agents, skills, and read-only permissions

.PHONY: sync-agents
sync-agents: ## Install Tide's portable agents into ~/.claude/agents/
	@./scripts/sync-agents.sh --target ~/ --categories portable --force

.PHONY: sync-skills
sync-skills: ## Install Tide's portable skills into ~/.claude/skills/
	@./scripts/sync-skills.sh --target ~/ --categories portable --force

.PHONY: sync-doctrine-self
sync-doctrine-self: ## Re-inject the operating-doctrine block into this repo's own AGENTS.md (dogfood; run after editing scripts/tide-doctrine.md)
	@bash -c '. ./scripts/lib/inject-doctrine.sh && inject_doctrine "." "./scripts/tide-doctrine.md" write'

.PHONY: sync-doctrine-self-check
sync-doctrine-self-check: ## Fail if this repo's AGENTS.md doctrine block has drifted from scripts/tide-doctrine.md (read-only; CI guard)
	@bash -c '. ./scripts/lib/inject-doctrine.sh && inject_doctrine "." "./scripts/tide-doctrine.md" check' \
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
