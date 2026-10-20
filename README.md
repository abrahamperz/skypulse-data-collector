# SkyPulse Data Collector

A small Go program that fetches live aircraft states from the **OpenSky Network** public API and stores them in a **SQLite** database. It is the data collection component (DC) of the SkyPulse project.

The program:

1. opens a SQLite database (creating the file if it does not exist),
2. applies the schema migrations in `migrations/`,
3. fetches one snapshot of aircraft states from `GET https://opensky-network.org/api/states/all`,
4. stores the snapshot and every aircraft state in the database.

## Requirements

- Go 1.22 or newer. Nothing else: the SQLite driver is pure Go (`modernc.org/sqlite`), so there is no C compiler, no CGO, and no separate database server to install.
- Internet access (the program calls the public OpenSky API).

## Run

From the project root:

```sh
go mod tidy   # downloads the one dependency, first run only
go run .
```

Expected output:

```
opening database skypulse.db
applying migrations
fetching global aircraft states from OpenSky Network
fetched 8123 states (OpenSky time 2026-09-28T18:04:00Z)
stored snapshot #1 with 8123 flight states

Done. skypulse.db now holds 8123 flight-state rows across all snapshots.
```

Run it again and a second snapshot is added; the migrations are skipped because they were already applied.

### Options

| Flag / variable | Default | Purpose |
|---|---|---|
| `-db <path>` | `skypulse.db` | Path to the SQLite database file. |
| `-timeout <duration>` | `30s` | HTTP timeout for the OpenSky request. |
| `OPENSKY_LAMIN` / `OPENSKY_LOMIN` / `OPENSKY_LAMAX` / `OPENSKY_LOMAX` | (unset) | Optional bounding box to limit the query to one region. All four must be set. |

Example, limiting the query to (roughly) Central America:

```sh
OPENSKY_LAMIN=7 OPENSKY_LOMIN=-92 OPENSKY_LAMAX=18 OPENSKY_LOMAX=-77 go run .
```

## Inspect the data

If you have the `sqlite3` command-line tool:

```sh
sqlite3 skypulse.db "SELECT origin_country, COUNT(*) AS flights
                     FROM flight_states
                     GROUP BY origin_country
                     ORDER BY flights DESC
                     LIMIT 10;"
```

## Project layout

```
.
├── main.go                     entry point: open DB, migrate, fetch, store
├── go.mod
├── migrations/
│   └── 0001_init.sql           database schema (snapshots + flight_states)
└── internal/
    ├── opensky/client.go       OpenSky API client and JSON decoding
    └── storage/storage.go      SQLite access: Open, Migrate, SaveSnapshot
```

## Schema

`snapshots` holds one row per fetch; `flight_states` holds one row per aircraft in that fetch, linked by `snapshot_id`. A `schema_migrations` table records which migration files have been applied, so startup is idempotent. See `migrations/0001_init.sql` for the full definition.
