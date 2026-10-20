// Package handler is the Vercel serverless entry point for the SkyPulse live
// demo. On each request it fetches a fresh snapshot of aircraft states from the
// OpenSky Network and renders it as HTML (or JSON with ?format=json).
//
// It is deliberately stateless: Vercel functions are ephemeral and their
// filesystem is read-only, so nothing is persisted here. Persisting snapshots
// is the job of the command-line collector (see main.go), which writes to
// SQLite. A short in-memory cache coalesces bursts of requests to stay within
// OpenSky's anonymous rate limits.
package handler

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"arch3/internal/opensky"
	"arch3/internal/web"
)

const (
	fetchTimeout = 25 * time.Second
	cacheTTL     = 30 * time.Second
)

var (
	client = opensky.New(fetchTimeout)

	mu       sync.Mutex
	cached   *opensky.Snapshot
	cachedAt time.Time
)

// Handler is the function Vercel invokes for every incoming request.
func Handler(w http.ResponseWriter, r *http.Request) {
	bbox := bboxFromQuery(r)

	snap, err := snapshot(r.Context(), bbox)
	if err != nil {
		http.Error(w,
			"Could not reach the OpenSky Network right now (it rate-limits anonymous "+
				"requests). Please try again in a moment.\n\nDetail: "+err.Error(),
			http.StatusBadGateway)
		return
	}

	if r.URL.Query().Get("format") == "json" {
		web.RenderJSON(w, snap)
		return
	}
	web.RenderHTML(w, snap, scopeLabel(bbox))
}

// snapshot returns a recent global snapshot from the in-memory cache when
// possible; regional (bounding-box) queries are always fetched fresh.
func snapshot(ctx context.Context, bbox opensky.BBox) (*opensky.Snapshot, error) {
	if !bbox.Set {
		mu.Lock()
		if cached != nil && time.Since(cachedAt) < cacheTTL {
			s := cached
			mu.Unlock()
			return s, nil
		}
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	snap, err := client.FetchStates(ctx, bbox)
	if err != nil {
		return nil, err
	}

	if !bbox.Set {
		mu.Lock()
		cached, cachedAt = snap, time.Now()
		mu.Unlock()
	}
	return snap, nil
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
