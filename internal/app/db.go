package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Host struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Discovered  time.Time `json:"discovered"`
	LastStatus  string    `json:"lastStatus"`
	LastCommit  string    `json:"lastCommit"`
	LastBuildID int64     `json:"lastBuildId"`
}

type Build struct {
	ID               int64      `json:"id"`
	Host             string     `json:"host"`
	CommitHash       string     `json:"commitHash"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	ExitCode         *int       `json:"exitCode"`
	OutputPath       string     `json:"outputPath"`
	Log              string     `json:"log"`
	Manual           bool       `json:"manual"`
	NotificationSent bool       `json:"notificationSent"`
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`create table if not exists settings (key text primary key, value text not null)`,
		`create table if not exists hosts (name text primary key, enabled integer not null default 0, discovered_at text not null)`,
		`create table if not exists builds (id integer primary key autoincrement, host text not null, commit_hash text not null, status text not null, started_at text not null, finished_at text, exit_code integer, output_path text not null default '', log text not null default '', manual integer not null default 0, notification_sent integer not null default 0)`,
		`create table if not exists sessions (token_hash text primary key, created_at text not null, expires_at text not null)`,
		`create index if not exists builds_host_commit on builds(host, commit_hash)`,
		`create index if not exists builds_started_at on builds(started_at desc)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `select value from settings where key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `insert into settings(key, value) values(?, ?) on conflict(key) do update set value = excluded.value`, key, value)
	return err
}

func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `delete from settings where key = ?`, key)
	return err
}

func (s *Store) UpsertHosts(ctx context.Context, names []string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, name := range names {
		if _, err := tx.ExecContext(ctx, `insert into hosts(name, enabled, discovered_at) values(?, 0, ?) on conflict(name) do update set discovered_at = excluded.discovered_at`, name, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Hosts(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx, `
		select h.name, h.enabled, h.discovered_at,
		       coalesce(b.status, ''), coalesce(b.commit_hash, ''), coalesce(b.id, 0)
		from hosts h
		left join builds b on b.id = (select id from builds where host = h.name order by started_at desc limit 1)
		order by h.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []Host
	for rows.Next() {
		var h Host
		var enabled int
		var discovered string
		if err := rows.Scan(&h.Name, &enabled, &discovered, &h.LastStatus, &h.LastCommit, &h.LastBuildID); err != nil {
			return nil, err
		}
		h.Enabled = enabled == 1
		h.Discovered, _ = time.Parse(time.RFC3339Nano, discovered)
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *Store) EnabledHosts(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select name from hosts where enabled = 1 order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []string
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

func (s *Store) SetHostEnabled(ctx context.Context, name string, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := s.db.ExecContext(ctx, `update hosts set enabled = ? where name = ?`, value, name)
	return err
}

func (s *Store) LatestBuildFor(ctx context.Context, host, commit string) (*Build, error) {
	row := s.db.QueryRowContext(ctx, `select id, host, commit_hash, status, started_at, finished_at, exit_code, output_path, log, manual, notification_sent from builds where host = ? and commit_hash = ? order by started_at desc limit 1`, host, commit)
	return scanBuild(row)
}

func (s *Store) CreateBuild(ctx context.Context, host, commit, status string, manual bool) (int64, error) {
	manualInt := 0
	if manual {
		manualInt = 1
	}
	res, err := s.db.ExecContext(ctx, `insert into builds(host, commit_hash, status, started_at, manual) values(?, ?, ?, ?, ?)`, host, commit, status, time.Now().UTC().Format(time.RFC3339Nano), manualInt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishBuild(ctx context.Context, id int64, status string, exitCode *int, outputPath, logText string) error {
	var exit any
	if exitCode != nil {
		exit = *exitCode
	}
	_, err := s.db.ExecContext(ctx, `update builds set status = ?, finished_at = ?, exit_code = ?, output_path = ?, log = ? where id = ?`, status, time.Now().UTC().Format(time.RFC3339Nano), exit, outputPath, logText, id)
	return err
}

func (s *Store) MarkNotificationSent(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `update builds set notification_sent = 1 where id = ?`, id)
	return err
}

func (s *Store) Build(ctx context.Context, id int64) (*Build, error) {
	row := s.db.QueryRowContext(ctx, `select id, host, commit_hash, status, started_at, finished_at, exit_code, output_path, log, manual, notification_sent from builds where id = ?`, id)
	return scanBuild(row)
}

func (s *Store) Builds(ctx context.Context, limit int) ([]Build, error) {
	rows, err := s.db.QueryContext(ctx, `select id, host, commit_hash, status, started_at, finished_at, exit_code, output_path, log, manual, notification_sent from builds order by started_at desc limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var builds []Build
	for rows.Next() {
		b, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}
		builds = append(builds, *b)
	}
	return builds, rows.Err()
}

type buildScanner interface {
	Scan(dest ...any) error
}

func scanBuild(scanner buildScanner) (*Build, error) {
	var b Build
	var started, finished sql.NullString
	var exit sql.NullInt64
	var manual, notification int
	if err := scanner.Scan(&b.ID, &b.Host, &b.CommitHash, &b.Status, &started, &finished, &exit, &b.OutputPath, &b.Log, &manual, &notification); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if started.Valid {
		b.StartedAt, _ = time.Parse(time.RFC3339Nano, started.String)
	}
	if finished.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, finished.String)
		b.FinishedAt = &parsed
	}
	if exit.Valid {
		code := int(exit.Int64)
		b.ExitCode = &code
	}
	b.Manual = manual == 1
	b.NotificationSent = notification == 1
	return &b, nil
}

func (b Build) ShortCommit() string {
	if len(b.CommitHash) <= 12 {
		return b.CommitHash
	}
	return b.CommitHash[:12]
}

func (b Build) Duration() string {
	if b.FinishedAt == nil {
		return time.Since(b.StartedAt).Round(time.Second).String()
	}
	return b.FinishedAt.Sub(b.StartedAt).Round(time.Second).String()
}

func (b Build) ExitCodeString() string {
	if b.ExitCode == nil {
		return ""
	}
	return fmt.Sprintf("%d", *b.ExitCode)
}
