package storage

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"arch3/internal/opensky"
)

// testMigrations is a self-contained migration set so the tests run offline
// and do not depend on the repository's migrations/ directory layout.
var testMigrations = fstest.MapFS{
	"migrations/0001_init.sql": {Data: []byte(`
		CREATE TABLE snapshots (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			fetched_at INTEGER NOT NULL,
			api_time   INTEGER NOT NULL
		);
		CREATE TABLE flight_states (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
			icao24      TEXT NOT NULL,
			callsign    TEXT,
			origin_country TEXT,
			time_position  INTEGER,
			last_contact   INTEGER,
			longitude   REAL,
			latitude    REAL,
			baro_altitude REAL,
			on_ground   INTEGER NOT NULL DEFAULT 0,
			velocity    REAL,
			true_track  REAL,
			vertical_rate REAL,
			geo_altitude REAL,
			squawk      TEXT,
			spi         INTEGER NOT NULL DEFAULT 0,
			position_source INTEGER
		);`)},
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// F6: migrations are applied once and re-running Migrate is a safe no-op.
func TestMigrate_Idempotent(t *testing.T) {
	store := newTestStore(t)

	if err := store.Migrate(testMigrations); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := store.Migrate(testMigrations); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var applied int
	if err := store.db.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations`,
	).Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 1 {
		t.Errorf("schema_migrations has %d rows, want 1 after two runs", applied)
	}
}

// F4/F5: SaveSnapshot writes one snapshot row plus one row per state, linked.
func TestSaveSnapshot(t *testing.T) {
	store := newTestStore(t)
	if err := store.Migrate(testMigrations); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	call := "DLH123"
	snap := &opensky.Snapshot{
		Time:      1700000000,
		FetchedAt: 1700000005,
		States: []opensky.State{
			{ICAO24: "abc123", Callsign: &call, OriginCountry: "Germany"},
			{ICAO24: "def456", OriginCountry: "France", OnGround: true},
		},
	}

	count, snapID, err := store.SaveSnapshot(snap)
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	if count != 2 {
		t.Errorf("stored %d states, want 2", count)
	}
	if snapID != 1 {
		t.Errorf("snapshot id = %d, want 1", snapID)
	}

	total, err := store.CountStates()
	if err != nil {
		t.Fatalf("count states: %v", err)
	}
	if total != 2 {
		t.Errorf("CountStates = %d, want 2", total)
	}

	// Every flight_states row must point at the snapshot we just wrote.
	var linked int
	if err := store.db.QueryRow(
		`SELECT COUNT(*) FROM flight_states WHERE snapshot_id = ?`, snapID,
	).Scan(&linked); err != nil {
		t.Fatalf("count linked: %v", err)
	}
	if linked != 2 {
		t.Errorf("%d rows linked to snapshot %d, want 2", linked, snapID)
	}
}
