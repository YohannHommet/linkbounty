package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type JobStatus string

const (
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusError   JobStatus = "error"
)

type Store struct {
	db *sql.DB
}

type Job struct {
	ID            string
	Domain        string
	Status        JobStatus
	ErrorMsg      string
	PagesCrawled  int
	BrokenCount   int
	ExtLinksFound int
	CreatedAt     time.Time
}

type BrokenLink struct {
	SourcePage string
	TargetLink string
	StatusCode int
	ErrorMsg   string
	LinkType   string // "broken" | "redirect" | "unverifiable"
}

type ReportGroup struct {
	SourcePage string
	Links      []BrokenLink
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Single writer: SQLite WAL serialises writes anyway; one open connection
	// avoids SQLITE_BUSY contention from the connection pool attempting parallel writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := configure(db); err != nil {
		return nil, fmt.Errorf("configure db: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return &Store{db: db}, nil
}

func configure(db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA wal_autocheckpoint=100`,
		`PRAGMA busy_timeout=5000`, // wait up to 5 s before returning SQLITE_BUSY
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

// migrations is the ordered list of schema changes. Append only — never edit existing entries.
var migrations = []string{
	// v1 — initial schema
	`CREATE TABLE IF NOT EXISTS jobs (
		id              TEXT PRIMARY KEY,
		domain          TEXT NOT NULL,
		status          TEXT NOT NULL DEFAULT 'running',
		error_msg       TEXT,
		pages_crawled   INTEGER NOT NULL DEFAULT 0,
		broken_count    INTEGER NOT NULL DEFAULT 0,
		ext_links_found INTEGER NOT NULL DEFAULT 0,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS broken_links (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id      TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
		source_page TEXT NOT NULL,
		target_link TEXT NOT NULL,
		status_code INTEGER NOT NULL DEFAULT 0,
		error_msg   TEXT,
		link_type   TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_broken_links_job_id ON broken_links(job_id)`,
}

func migrate(db *sql.DB) error {
	// Use SQLite's built-in user_version pragma as a schema version counter.
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if _, err := db.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		// user_version cannot be set via a parameterised query
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			return fmt.Errorf("set schema version %d: %w", i+1, err)
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateJob(id, domain string) error {
	_, err := s.db.Exec(
		`INSERT INTO jobs(id, domain, status) VALUES(?, ?, 'running')`,
		id, domain,
	)
	return err
}

func (s *Store) UpdateJobDone(id string, pagesCrawled, brokenCount, extLinksFound int) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status='done', pages_crawled=?, broken_count=?, ext_links_found=? WHERE id=?`,
		pagesCrawled, brokenCount, extLinksFound, id,
	)
	return err
}

func (s *Store) UpdateJobError(id, msg string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status='error', error_msg=? WHERE id=?`,
		msg, id,
	)
	return err
}

func (s *Store) MarkOrphansError() error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status='error', error_msg='Server restarted' WHERE status='running'`,
	)
	return err
}

func (s *Store) SaveLinks(jobID string, links []BrokenLink) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO broken_links(job_id, source_page, target_link, status_code, error_msg, link_type)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, l := range links {
		if _, err := stmt.Exec(jobID, l.SourcePage, l.TargetLink, l.StatusCode, l.ErrorMsg, l.LinkType); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetJob(id string) (*Job, error) {
	row := s.db.QueryRow(
		`SELECT id, domain, status, COALESCE(error_msg,''), pages_crawled, broken_count, ext_links_found, created_at FROM jobs WHERE id=?`,
		id,
	)
	var j Job
	if err := row.Scan(&j.ID, &j.Domain, &j.Status, &j.ErrorMsg, &j.PagesCrawled, &j.BrokenCount, &j.ExtLinksFound, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (s *Store) GetReportGroups(jobID string) ([]ReportGroup, error) {
	rows, err := s.db.Query(`
		SELECT source_page, target_link, status_code, COALESCE(error_msg,''), link_type
		FROM broken_links
		WHERE job_id=?
		ORDER BY source_page, link_type, target_link
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[string]*ReportGroup{}
	var order []string

	for rows.Next() {
		var l BrokenLink
		var src string
		if err := rows.Scan(&src, &l.TargetLink, &l.StatusCode, &l.ErrorMsg, &l.LinkType); err != nil {
			return nil, err
		}
		l.SourcePage = src
		if _, ok := grouped[src]; !ok {
			grouped[src] = &ReportGroup{SourcePage: src}
			order = append(order, src)
		}
		grouped[src].Links = append(grouped[src].Links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]ReportGroup, 0, len(order))
	for _, src := range order {
		result = append(result, *grouped[src])
	}
	return result, nil
}

func (s *Store) Cleanup(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	_, err := s.db.Exec(`DELETE FROM jobs WHERE created_at < ?`, cutoff)
	return err
}
