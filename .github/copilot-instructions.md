# Rule 1: git-lrc build command

Build and install lrc locally with:

make build-local && lrc hooks install

When building the lrc command, always use the binary name "lrc"

A primary rule in programming is that the name must match function or meaning. If the name says one thing and the function or meaning is another - it must be treated as a major bug and treated with highest priority. Always fix naming bugs immediately when discovered.

When doing refactors or improvements - don't do fallback implementations unless explicitly asked for. Having fallbacks creates very confusing program behavior.

When you want to run a test - always run the most specific test you want to run. Don't run all or other irrelevant tests.

When making proposals - think of how you can accomplish goals in an incremental way rather than suggesting sweeping refactors.


# Rule 2: Code organization and status docs

Enforces the rule that db, file operations go into storage folder/module with specific function/struct names in logical files. Then network operations go into network folder/module again with specific function/struct names in logical files.

The other code should call these storage/network functions/structs and use the results from there.

This should be enforced and not violated.

Secondly, the status docs #file:storage_status.md   and #file:network_status.md  be also simultaneously updated to keep with the new changes.

Finally the line numbers should be kept upto date in these status docs, and violation should be checked via #file:check-status-doc-links.sh 

Stick strictly to my language and specifications here. Don't change it and generalize it the way you want. Stay with my guidance as is.


# Rule 3: UI iconography guidance

For any UI icon work in git-lrc, follow `docs/ui-iconography.md` as the source of truth.

- Use the shared icon registry in `internal/staticserve/static/components/icons.js`.
- Do not introduce new emoji or Unicode action icons.
- Do not force vendor logos onto action buttons just because the label names a vendor.
- If a new icon decision is needed, update `docs/ui-iconography.md` together with the code.