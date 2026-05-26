__LRC_MARKER_BEGIN__
# lrc_version: __LRC_VERSION__
# LiveReview global dispatcher for __HOOK_NAME__
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GIT_DIR="$(git rev-parse --git-dir 2>/dev/null || echo .git)"
GIT_COMMON_DIR="$(git rev-parse --git-common-dir 2>/dev/null || echo "$GIT_DIR")"
LRC_DIR="$SCRIPT_DIR/lrc"
LRC_DISABLED_FILE="$GIT_DIR/lrc/disabled"
LRC_HOOK="$LRC_DIR/__HOOK_NAME__"
LOCAL_HOOK="$GIT_COMMON_DIR/hooks/__HOOK_NAME__"

if [ -f "$LRC_DISABLED_FILE" ]; then
	LRC_DISABLED=1
else
	LRC_DISABLED=0
fi

if [ $LRC_DISABLED -eq 0 ] && [ -x "$LRC_HOOK" ]; then
	"$LRC_HOOK" "$@"
	LRC_STATUS=$?
else
	LRC_STATUS=0
fi

if [ $LRC_STATUS -ne 0 ]; then
	exit $LRC_STATUS
fi

if [ -x "$LOCAL_HOOK" ]; then
	"$LOCAL_HOOK" "$@"
	LOCAL_STATUS=$?
	if [ $LOCAL_STATUS -ne 0 ]; then
		exit $LOCAL_STATUS
	fi
fi
__LRC_MARKER_END__
