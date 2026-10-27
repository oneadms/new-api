#!/usr/bin/env bash
# Rehearse on a disposable local database. Never use a production DSN.
set -euo pipefail

: "${UPGRADE_REHEARSAL_DSN:?Set a disposable local database DSN (or local for SQLite)}"
repo_root=$(git rev-parse --show-toplevel)
base_revision=${1:-dd9087206a868b89d05e35fba8d754efe27e75be}
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/new-api-upgrade-source.XXXXXX")
trap 'rm -rf "$fixture_dir"' EXIT

git -C "$repo_root" archive "$base_revision" | tar -x -C "$fixture_dir"
cp "$repo_root/model/upstream_upgrade_test.go" "$fixture_dir/model/upstream_upgrade_test.go"
(
  cd "$fixture_dir"
  UPGRADE_REHEARSAL_MODE=seed go test ./model -run '^TestUpstreamDatabaseUpgradeRehearsal$' -count=1 -v
)
(
  cd "$repo_root"
  UPGRADE_REHEARSAL_MODE=upgrade go test ./model -run '^TestUpstreamDatabaseUpgradeRehearsal$' -count=1 -v
)
