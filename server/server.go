// Package server holds the HTTP handler for the SkyPulse live demo. On each
// request it fetches a fresh snapshot of aircraft states from the OpenSky
// Network and renders it as HTML (or JSON with ?format=json).
//
// It is deliberately stateless: it persists nothing, so it runs cleanly in a
// stateless hosting environment. Persisting snapshots is the job of the
// command-line collector (see main.go in the repository root), which writes to
// SQLite. A short in-memory cache coalesces bursts of requests to stay within
// OpenSky's anonymous rate limits, and an embedded sample snapshot is served as
// a fallback when OpenSky cannot be reached.
package server

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"arch3/opensky"
	"arch3/web"
)

const (
	// fetchTimeout bounds the live OpenSky call. It is deliberately short so
	// that, when OpenSky is unreachable from the host, the page falls back to
	// sample data quickly instead of making the visitor wait.
	fetchTimeout = 6 * time.Second
	cacheTTL     = 30 * time.Second
)

var (
	client = opensky.New(fetchTimeout)

	mu         sync.Mutex
	cached     *opensky.Snapshot
	cachedAt   time.Time
	cachedLive bool
)

// Handler serves the live demo for every incoming request.
func Handler(w http.ResponseWriter, r *http.Request) {
	bbox := bboxFromQuery(r)
	snap, live := snapshot(r.Context(), bbox)

	if r.URL.Query().Get("format") == "json" {
		web.RenderJSON(w, snap)
		return
	}
	web.RenderHTML(w, snap, scopeLabel(bbox), live)
}

// snapshot returns a snapshot and whether it is live OpenSky data. When the
// live API cannot be reached (rate limit, or it blocks datacenter IPs) it falls
// back to the embedded sample snapshot. Global results, live or fallback, are
// cached for cacheTTL so bursts of requests are fast and OpenSky is not hammered;
// after the TTL the next request retries the live feed. Regional (bounding-box)
// queries are always fetched fresh.
func snapshot(ctx context.Context, bbox opensky.BBox) (*opensky.Snapshot, bool) {
	if !bbox.Set {
		mu.Lock()
		if cached != nil && time.Since(cachedAt) < cacheTTL {
			s, live := cached, cachedLive
			mu.Unlock()
			return s, live
		}
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	snap, err := client.FetchStates(ctx, bbox)
	live := err == nil
	if !live {
		snap = opensky.SampleSnapshot()
	}

	if !bbox.Set {
		mu.Lock()
		cached, cachedAt, cachedLive = snap, time.Now(), live
		mu.Unlock()
	}
	return snap, live
}

// bboxFromQuery reads an optional bounding box from the lamin/lomin/lamax/lomax
// query parameters. All four must be valid floats, otherwise the whole world
// is queried.
func bboxFromQuery(r *http.Request) opensky.BBox {
	q := r.URL.Query()
	get := func(k string) (float64, bool) {
		v := q.Get(k)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	laMin, ok1 := get("lamin")
	loMin, ok2 := get("lomin")
	laMax, ok3 := get("lamax")
	loMax, ok4 := get("lomax")
	if ok1 && ok2 && ok3 && ok4 {
		return opensky.BBox{LaMin: laMin, LoMin: loMin, LaMax: laMax, LoMax: loMax, Set: true}
	}
	return opensky.BBox{}
}

func scopeLabel(bbox opensky.BBox) string {
	if bbox.Set {
		return "regional bounding box"
	}
	return "global"
}
