package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// State represents the user-activity state for a session.
type State string

const (
	StateActive  State = "active"
	StateIdle    State = "idle"
	StateOffline State = "offline"
)

// Session is a contiguous block of time spent in a single State.
type Session struct {
	ID        int64
	MachineID string
	Date      string
	StartTime time.Time
	EndTime   time.Time
	State     State
	Synced    bool
	// Location fields (empty when no GPS fix at session start)
	Latitude  float64
	Longitude float64
	// VersionID is the release version of the binary that created this session.
	VersionID string
	// Network fields captured at session start
	PublicIP string
	SSID     string
}

// DB wraps a sql.DB for the tracker's usage pattern.
type DB struct {
	db *sql.DB
}

// Open creates directories, opens (or creates) the SQLite DB,
// and applies the schema.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("storage: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("storage: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

// Close releases the underlying connection.
func (d *DB) Close() error { return d.db.Close() }

// LocationInfo carries GPS coordinates and network info captured at session start.
type LocationInfo struct {
	Latitude  float64
	Longitude float64
	PublicIP  string
	SSID      string
}

// StartSession inserts a new open session and returns its ID.
func (d *DB) StartSession(machineID string, state State, at time.Time, loc LocationInfo, versionID string) (int64, error) {
	date := at.Local().Format("2006-01-02")
	res, err := d.db.Exec(
		`INSERT INTO sessions (machine_id, date, start_time, state, synced, latitude, longitude, version_id, public_ip, ssid)
		 VALUES (?,?,?,?,0,?,?,?,?,?)`,
		machineID, date, at.UTC().UnixNano(), string(state),
		loc.Latitude, loc.Longitude, versionID, loc.PublicIP, loc.SSID,
	)
	if err != nil {
		return 0, fmt.Errorf("storage: start session: %w", err)
	}
	return res.LastInsertId()
}

// CloseSession sets end_time on the open session with id.
func (d *DB) CloseSession(id int64, at time.Time) error {
	_, err := d.db.Exec(
		"UPDATE sessions SET end_time = ? WHERE id = ? AND end_time IS NULL",
		at.UTC().UnixNano(), id,
	)
	return err
}

// MarkSynced marks the given session IDs as synced.
func (d *DB) MarkSynced(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE sessions SET synced = 1 WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// DeleteSyncedBefore deletes synced sessions with date < date.
func (d *DB) DeleteSyncedBefore(date string) (int64, error) {
	res, err := d.db.Exec("DELETE FROM sessions WHERE synced = 1 AND date < ?", date)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteOlderThan removes all sessions older than retentionDays days.
func (d *DB) DeleteOlderThan(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02")
	res, err := d.db.Exec("DELETE FROM sessions WHERE date < ? AND synced = 1", cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// TouchSession updates last_heartbeat on an open session.
// Called on every poll tick so that if the process is killed, we have an
// accurate "last known alive" timestamp for crash recovery.
// NOTE: This writes to last_heartbeat, NOT end_time. end_time stays NULL
// for in-progress sessions so sync queries (which filter on end_time IS
// NOT NULL) don't accidentally pick up active sessions.
func (d *DB) TouchSession(id int64, at time.Time) error {
	_, err := d.db.Exec(
		"UPDATE sessions SET last_heartbeat = ? WHERE id = ?",
		at.UTC().UnixNano(), id,
	)
	return err
}

// CloseHangingSessions fixes sessions left open by a previous crash or
// SIGKILL. Uses the last_heartbeat timestamp (written every poll tick) as
// the best approximation of when the process was last alive. Falls back to
// start_time + fallbackBuffer if no heartbeat was ever recorded.
func (d *DB) CloseHangingSessions(fallbackBuffer time.Duration) (int64, error) {
	bufferNanos := int64(fallbackBuffer)
	res, err := d.db.Exec(
		"UPDATE sessions SET end_time = COALESCE(last_heartbeat, start_time + ?) WHERE end_time IS NULL",
		bufferNanos,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// UnsyncedForDate returns closed, unsynced sessions for a specific date.
func (d *DB) UnsyncedForDate(date string) ([]Session, error) {
	rows, err := d.db.Query(
		`SELECT id, machine_id, date, start_time, end_time, state, latitude, longitude, version_id, public_ip, ssid
			 FROM sessions WHERE date = ? AND synced = 0 AND end_time IS NOT NULL ORDER BY start_time`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// UnsyncedAll returns all closed, unsynced sessions (used by realtime sync).
func (d *DB) UnsyncedAll() ([]Session, error) {
	rows, err := d.db.Query(
		`SELECT id, machine_id, date, start_time, end_time, state, latitude, longitude, version_id, public_ip, ssid
			 FROM sessions WHERE synced = 0 AND end_time IS NOT NULL ORDER BY start_time`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// LastSyncedByDate returns the most recently ended synced session for each of
// the provided dates. The result is a map keyed by date string. Dates with no
// synced session are omitted from the map.
func (d *DB) LastSyncedByDate(dates []string) (map[string]Session, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	// Build a parameterised IN clause.
	placeholders := make([]string, len(dates))
	args := make([]interface{}, len(dates))
	for i, dt := range dates {
		placeholders[i] = "?"
		args[i] = dt
	}
	query := `SELECT id, machine_id, date, start_time, end_time, state, latitude, longitude, version_id, public_ip, ssid
		FROM sessions
		WHERE synced = 1 AND end_time IS NOT NULL AND date IN (` +
		strings.Join(placeholders, ",") + `)
		GROUP BY date
		HAVING end_time = MAX(end_time)
		ORDER BY date`
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions, err := scanSessions(rows)
	if err != nil {
		return nil, err
	}
	m := make(map[string]Session, len(sessions))
	for _, s := range sessions {
		m[s.Date] = s
	}
	return m, nil
}

// UnsyncedUpToDate returns closed, unsynced sessions with date <= date.
func (d *DB) UnsyncedUpToDate(date string) ([]Session, error) {
	rows, err := d.db.Query(
		`SELECT id, machine_id, date, start_time, end_time, state, latitude, longitude, version_id, public_ip, ssid
			 FROM sessions WHERE date <= ? AND synced = 0 AND end_time IS NOT NULL ORDER BY date, start_time`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// SessionCount returns the total session count for health logging.
func (d *DB) SessionCount() int64 {
	var n int64
	if err := d.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		log.Printf("storage: session count: %v", err)
	}
	return n
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		machine_id  TEXT    NOT NULL,
		date        TEXT    NOT NULL,
		start_time  INTEGER NOT NULL,
		end_time    INTEGER,
		state       TEXT    NOT NULL,
		synced      INTEGER NOT NULL DEFAULT 0,
		latitude    REAL,
		longitude   REAL
	)`)
	if err != nil {
		return fmt.Errorf("storage: migrate create table: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_date_synced ON sessions (date, synced)`)
	if err != nil {
		return fmt.Errorf("storage: migrate create index: %w", err)
	}
	// Add location columns to existing databases (safe to ignore duplicate-column errors).
	db.Exec(`ALTER TABLE sessions ADD COLUMN latitude REAL`)
	db.Exec(`ALTER TABLE sessions ADD COLUMN longitude REAL`)
	// Add version_id column (safe to ignore duplicate-column errors).
	db.Exec(`ALTER TABLE sessions ADD COLUMN version_id TEXT`)
	// Add last_heartbeat column for crash recovery (safe to ignore duplicate-column errors).
	db.Exec(`ALTER TABLE sessions ADD COLUMN last_heartbeat INTEGER`)
	// Add network info columns (safe to ignore duplicate-column errors).
	db.Exec(`ALTER TABLE sessions ADD COLUMN public_ip TEXT`)
	db.Exec(`ALTER TABLE sessions ADD COLUMN ssid TEXT`)
	return nil
}

func scanSessions(rows *sql.Rows) ([]Session, error) {
	var out []Session
	for rows.Next() {
		var s Session
		var startNano int64
		var endNano sql.NullInt64
		var lat, lon sql.NullFloat64
		var versionID, publicIP, ssid sql.NullString
		if err := rows.Scan(&s.ID, &s.MachineID, &s.Date, &startNano, &endNano, &s.State, &lat, &lon, &versionID, &publicIP, &ssid); err != nil {
			return nil, err
		}
		s.StartTime = time.Unix(0, startNano).UTC()
		if endNano.Valid {
			s.EndTime = time.Unix(0, endNano.Int64).UTC()
		}
		s.Latitude = lat.Float64
		s.Longitude = lon.Float64
		s.VersionID = versionID.String
		s.PublicIP = publicIP.String
		s.SSID = ssid.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// LastSession returns the most recent session by start_time.
func (d *DB) LastSession() (*Session, error) {
	var s Session
	var startNano int64
	var endNano sql.NullInt64
	var lat, lon sql.NullFloat64
	var versionID, publicIP, ssid sql.NullString
	err := d.db.QueryRow(
		`SELECT id, machine_id, date, start_time, end_time, state, latitude, longitude, version_id, public_ip, ssid
		 FROM sessions ORDER BY start_time DESC LIMIT 1`,
	).Scan(&s.ID, &s.MachineID, &s.Date, &startNano, &endNano, &s.State, &lat, &lon, &versionID, &publicIP, &ssid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.StartTime = time.Unix(0, startNano).UTC()
	if endNano.Valid {
		s.EndTime = time.Unix(0, endNano.Int64).UTC()
	}
	s.Latitude = lat.Float64
	s.Longitude = lon.Float64
	s.VersionID = versionID.String
	s.PublicIP = publicIP.String
	s.SSID = ssid.String
	return &s, nil
}

