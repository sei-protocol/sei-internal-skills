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
bootstrap: sync-agents update-agent-permissions ## One-shot: install agents and read-only permissions

.PHONY: sync-agents
sync-agents: ## Install Tide's portable agents into ~/.claude/agents/
	@./scripts/sync-agents.sh --target ~/ --categories portable --force

.PHONY: update-agent-permissions
update-agent-permissions: ## Install canonical read-only allow-list into ./.claude/settings.json (DRY_RUN=1 to preview)
	@./scripts/update-agent-permissions.sh

.PHONY: verify-agent-permissions
verify-agent-permissions: ## Fail if .claude/settings.json contains mutating patterns or has drifted
	@./scripts/verify-agent-permissions.sh
