// Package storage owns the SQLite database: opening it, applying schema
// migrations, and writing the snapshots produced by the collector.
package storage

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"arch3/internal/opensky"

	// Pure-Go SQLite driver (no CGO, no C compiler required).
	_ "modernc.org/sqlite"
)

// Store wraps the database handle.
type Store struct {
	db *sql.DB
}

// Open opens (creating it if needed) the SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate applies every migrations/*.sql file, in filename order, that has
// not been applied yet. Applied versions are recorded in schema_migrations
// so re-running the program is safe (idempotent).
func (s *Store) Migrate(files fs.FS) error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name,
		).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}

		content, err := fs.ReadFile(files, "migrations/"+name)
		if err != nil {
			return err
		}

		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			name, time.Now().Unix(),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// SaveSnapshot writes one snapshot and all its flight states in a single
// transaction. It returns the number of states written and the new snapshot id.
func (s *Store) SaveSnapshot(snap *opensky.Snapshot) (int, int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO snapshots (fetched_at, api_time) VALUES (?, ?)`,
		snap.FetchedAt, snap.Time,
	)
	if err != nil {
		return 0, 0, err
	}
	snapshotID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO flight_states (
			snapshot_id, icao24, callsign, origin_country, time_position, last_contact,
			longitude, latitude, baro_altitude, on_ground, velocity, true_track,
			vertical_rate, geo_altitude, squawk, spi, position_source
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	count := 0
	for _, st := range snap.States {
		if _, err := stmt.Exec(
			snapshotID, st.ICAO24, st.Callsign, st.OriginCountry, st.TimePosition, st.LastContact,
			st.Longitude, st.Latitude, st.BaroAltitude, boolToInt(st.OnGround), st.Velocity, st.TrueTrack,
			st.VerticalRate, st.GeoAltitude, st.Squawk, boolToInt(st.SPI), st.PositionSource,
		); err != nil {
			return 0, 0, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return count, snapshotID, nil
}

// CountStates returns the total number of flight-state rows across all snapshots.
func (s *Store) CountStates() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM flight_states`).Scan(&n)
	return n, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
