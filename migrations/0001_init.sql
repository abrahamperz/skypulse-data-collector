-- 0001_init.sql
-- SkyPulse Data Collector: initial schema.
-- One row in `snapshots` per fetch from OpenSky; many `flight_states` per snapshot.

CREATE TABLE IF NOT EXISTS snapshots (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    fetched_at INTEGER NOT NULL,  -- unix epoch seconds when the collector ran
    api_time   INTEGER NOT NULL   -- "time" reported by the OpenSky response
);

CREATE TABLE IF NOT EXISTS flight_states (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id     INTEGER NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    icao24          TEXT    NOT NULL,           -- unique ICAO 24-bit address
    callsign        TEXT,                       -- flight callsign (may be null)
    origin_country  TEXT,
    time_position   INTEGER,                    -- unix time of last position update
    last_contact    INTEGER,                    -- unix time of last message received
    longitude       REAL,
    latitude        REAL,
    baro_altitude   REAL,                       -- barometric altitude in meters
    on_ground       INTEGER NOT NULL DEFAULT 0, -- 0 = airborne, 1 = on ground
    velocity        REAL,                       -- speed over ground in m/s
    true_track      REAL,                       -- heading in decimal degrees
    vertical_rate   REAL,                       -- m/s (positive = climbing)
    geo_altitude    REAL,                       -- geometric altitude in meters
    squawk          TEXT,
    spi             INTEGER NOT NULL DEFAULT 0, -- special purpose indicator
    position_source INTEGER
);

CREATE INDEX IF NOT EXISTS idx_flight_states_snapshot ON flight_states(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_flight_states_icao24   ON flight_states(icao24);
CREATE INDEX IF NOT EXISTS idx_flight_states_country  ON flight_states(origin_country);
