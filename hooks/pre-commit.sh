__LRC_MARKER_BEGIN__
# lrc_version: __LRC_VERSION__
# This section is managed by LiveReview CLI (lrc)
# Manual changes within markers will be lost on hook updates

# Resolve repository state paths through Git so linked worktrees use their per-worktree git dir.
GIT_DIR="$(git rev-parse --git-dir 2>/dev/null || echo .git)"
LRC_DIR="$GIT_DIR/lrc"
DISABLED_FILE="$LRC_DIR/disabled"
if [ -f "$DISABLED_FILE" ]; then
	exit 0
fi

# Skip during Git sequencer operations to avoid re-triggering on rebase/merge/cherry-pick
if [ -d "$GIT_DIR/rebase-apply" ] || [ -d "$GIT_DIR/rebase-merge" ] || [ -f "$GIT_DIR/MERGE_HEAD" ] || [ -f "$GIT_DIR/CHERRY_PICK_HEAD" ]; then
	echo "LiveReview: skipping during rebase/merge/cherry-pick" >&2
	exit 0
fi

# Detect interactive terminal (stdout check; git redirects stdin)
if [ -t 1 ]; then
	echo "LiveReview pre-commit: interactive environment detected; no-op" >&2
	exit 0
fi

# Non-interactive: require attestation for current staged tree
TREE_HASH="$(git write-tree 2>/dev/null || true)"
ATTEST_FILE="$LRC_DIR/attestations/$TREE_HASH.json"

if [ -z "$TREE_HASH" ]; then
	echo "LiveReview: failed to compute staged tree hash. Run 'lrc review --staged' and retry commit." >&2
	exit 1
fi

if [ ! -f "$ATTEST_FILE" ]; then
	echo "LiveReview: review attestation missing for staged changes. Run 'lrc review --staged' (or --skip/--vouch) and retry commit." >&2
	exit 1
fi

echo "LiveReview pre-commit: attestation present for $TREE_HASH; proceeding"
exit 0
__LRC_MARKER_END__
