package storage

import (
	"database/sql"
	"fmt"
)

// QueryReviewSessionCountByBranch returns the number of review sessions for a branch.
func QueryReviewSessionCountByBranch(db *sql.DB, branch string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM review_sessions WHERE branch = ?`, branch).Scan(&count)
	return count, err
}

// QueryReviewedSessionsByBranch returns reviewed (and agent-reviewed)
// sessions for a branch in timestamp order. Callers must close the returned
// rows.
func QueryReviewedSessionsByBranch(db *sql.DB, branch string) (*sql.Rows, error) {
	rows, err := db.Query(
		`SELECT id, tree_hash, branch, action, timestamp, diff_files, review_id
		 FROM review_sessions
		 WHERE branch = ? AND action IN ('reviewed', 'agent-reviewed')
		 ORDER BY timestamp ASC`,
		branch,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviewed sessions for branch %q: %w", branch, err)
	}
	return rows, nil
}
