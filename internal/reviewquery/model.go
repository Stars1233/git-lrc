// Package reviewquery builds a queryable view of a repo's LiveReview history.
//
// It extracts review metadata from git commit trailers into structured records,
// loads them into an in-memory SQLite table, and runs SQL queries (or named
// aliases) against that table — the "filter -> group -> aggregate" engine.
package reviewquery

import "time"

// ReviewRecord is one commit's review metadata, one row in the review_log table.
type ReviewRecord struct {
	Hash        string    // full commit hash
	ShortHash   string    // abbreviated hash
	Author      string    // author name
	Email       string    // author email
	Date        time.Time // author date
	Branch      string    // branch the query was run from (Phase 1: current branch)
	Subject     string    // commit subject (first line)
	Action      string    // reviewed | vouched | skipped | none
	Iterations  int       // review iterations (0 if absent)
	CoveragePct int       // coverage percent (0 if absent)
}

// Filter narrows which commits are extracted. Phase 1 uses Range/Since; the
// rest are wired in Phase 2.
type Filter struct {
	Range      string    // e.g. "main...feature" (PR diff); empty = full history
	Since      time.Time // zero = no lower bound
	Until      time.Time // zero = no upper bound
	Author     string    // substring match on author/email
	PathPrefix string    // limit to commits touching this path
	Action     string    // limit to one action
}

// Alias is a saved, named SQL query stored in ~/.lrc/queries.toml.
type Alias struct {
	Name string `toml:"name"`
	SQL  string `toml:"sql"`
}
