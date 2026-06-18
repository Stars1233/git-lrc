package reviewquery

import (
	"fmt"

	"github.com/HexmosTech/git-lrc/storage"
)

// QueryResult is a generic tabular result: column headers + stringified rows.
type QueryResult struct {
	Columns []string
	Rows    [][]string
}

const createTableSQL = `
CREATE TABLE review_log (
    hash         TEXT,
    short_hash   TEXT,
    author       TEXT,
    email        TEXT,
    date         TEXT,
    branch       TEXT,
    subject      TEXT,
    action       TEXT,
    iterations   INTEGER,
    coverage     INTEGER
);`

const insertSQL = `
INSERT INTO review_log
    (hash, short_hash, author, email, date, branch, subject, action, iterations, coverage)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

// Run extracts the review history (scoped by filter), loads it into an in-memory
// SQLite table named review_log, and runs sqlText against it. sqlText must
// already be resolved (alias -> SQL) by the caller.
func Run(f Filter, sqlText string) (QueryResult, error) {
	records, err := Extract(f)
	if err != nil {
		return QueryResult{}, err
	}
	return RunOnRecords(records, sqlText)
}

// RunOnRecords loads records into an in-memory review_log table and runs sqlText
// against it. Split out from Run so it can be tested/benchmarked without git.
func RunOnRecords(records []ReviewRecord, sqlText string) (QueryResult, error) {
	db, err := storage.OpenInMemorySQLite()
	if err != nil {
		return QueryResult{}, err
	}
	defer func() { _ = db.Close() }()

	if _, err := storage.ExecSQL(db, createTableSQL); err != nil {
		return QueryResult{}, fmt.Errorf("failed to create review_log table: %w", err)
	}

	rows := make([][]any, 0, len(records))
	for _, r := range records {
		date := ""
		if !r.Date.IsZero() {
			date = r.Date.UTC().Format("2006-01-02T15:04:05Z")
		}
		rows = append(rows, []any{
			r.Hash, r.ShortHash, r.Author, r.Email, date,
			r.Branch, r.Subject, r.Action, r.Iterations, r.CoveragePct,
		})
	}
	if err := storage.BulkInsert(db, insertSQL, rows); err != nil {
		return QueryResult{}, err
	}

	columns, outRows, err := storage.QueryRows(db, sqlText)
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Columns: columns, Rows: outRows}, nil
}
