#!/bin/sh
# legacycoin-suite: update all forks-ecosystem projects under /app from git,
# then rebuild the suite container.
#
# Safe by design:
#   - fast-forward only (--ff-only); never resets, stashes or force-pulls
#   - skips repos that have tracked local changes
#   - untracked files do not block the update
#
# Other projects (forkex, pool, explorer, ...) are only *pulled* here;
# rebuild and restart each one with its own procedure.

set -u

ROOT="${APP_ROOT:-/app}"
FAILED=""

echo "=== [1/2] Updating git repos under $ROOT ==="
for d in "$ROOT"/*/; do
    [ -d "$d/.git" ] || continue
    name=$(basename "$d")

    if [ -n "$(git -C "$d" status --porcelain --untracked-files=no 2>/dev/null)" ]; then
        echo "  [skip] $name — has local (tracked) changes; commit or stash first"
        continue
    fi

    echo "--> $name"
    out=$(git -C "$d" pull --ff-only 2>&1)
    rc=$?
    printf '%s\n' "$out" | sed 's/^/    /'
    if [ "$rc" -ne 0 ]; then
        echo "    !! pull failed"
        FAILED="$FAILED $name"
    fi
done

echo
echo "=== [2/2] Rebuilding legacycoin-suite ==="
if [ -x "$ROOT/legacycoin-suite/__rebuild.sh" ]; then
    "$ROOT/legacycoin-suite/__rebuild.sh"
else
    echo "!! $ROOT/legacycoin-suite/__rebuild.sh not found"
fi

echo
if [ -n "$FAILED" ]; then
    echo "!! pull failed for:$FAILED"
    exit 1
fi
echo "==> Done. (Rebuild/restart other projects if they changed.)"
