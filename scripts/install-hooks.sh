#!/usr/bin/env bash
set -euo pipefail

# Installs this repo's tracked git hooks (.githooks/) into .git/hooks/, so
# go test ./... runs automatically before every push. See CLAUDE.md §8.

REPO_ROOT="$(git rev-parse --show-toplevel)"

for hook in "$REPO_ROOT"/.githooks/*; do
	name="$(basename "$hook")"
	install -m 0755 "$hook" "$REPO_ROOT/.git/hooks/$name"
	echo "installed $name hook"
done
