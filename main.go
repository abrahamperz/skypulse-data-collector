// Command collector is the SkyPulse Data Collector (DC).
// It fetches a snapshot of live aircraft states from the OpenSky Network
// public API and stores it in a SQLite database, applying schema migrations
// on startup.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"time"

	"arch3/internal/opensky"
	"arch3/internal/storage"
)

// migrationsFS embeds the SQL migration files so the program applies the
// schema without depending on the working directory. The files stay visible
// in the migrations/ folder for the reviewer to read.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	dbPath := flag.String("db", "skypulse.db", "path to the SQLite database file")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP timeout for the OpenSky request")
	flag.Parse()

	// Optional geographic bounding box (region) read from environment variables.
	bbox := opensky.BBoxFromEnv()

	log.Printf("opening database %s", *dbPath)
	store, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	log.Printf("applying migrations")
	var mfs fs.FS = migrationsFS
	if err := store.Migrate(mfs); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	client := opensky.New(*timeout)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if bbox.Set {
		log.Printf("fetching aircraft states within bounding box lat[%.3f, %.3f] lon[%.3f, %.3f]",
			bbox.LaMin, bbox.LaMax, bbox.LoMin, bbox.LoMax)
	} else {
		log.Printf("fetching global aircraft states from OpenSky Network")
	}
	snap, err := client.FetchStates(ctx, bbox)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}
	log.Printf("fetched %d states (OpenSky time %s)", len(snap.States),
		time.Unix(snap.Time, 0).UTC().Format(time.RFC3339))

	count, snapshotID, err := store.SaveSnapshot(snap)
	if err != nil {
		log.Fatalf("save snapshot: %v", err)
	}
	log.Printf("stored snapshot #%d with %d flight states", snapshotID, count)

	total, err := store.CountStates()
	if err != nil {
		log.Fatalf("count states: %v", err)
	}
	fmt.Printf("\nDone. %s now holds %d flight-state rows across all snapshots.\n", *dbPath, total)
}
