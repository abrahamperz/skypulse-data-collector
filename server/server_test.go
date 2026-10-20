package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arch3/opensky"
)

// fakeFetcher stands in for the real OpenSky client so tests run offline.
type fakeFetcher struct {
	snap *opensky.Snapshot
	err  error
}

func (f fakeFetcher) FetchStates(context.Context, opensky.BBox) (*opensky.Snapshot, error) {
	return f.snap, f.err
}

// use swaps in a fake fetcher and clears the cache so each test starts clean.
func use(f fetcher) {
	client = f
	mu.Lock()
	cached, cachedLive = nil, false
	mu.Unlock()
}

func liveSnapshot() *opensky.Snapshot {
	call := "DLH1"
	return &opensky.Snapshot{
		Time: 1700000000,
		States: []opensky.State{
			{ICAO24: "aaa111", Callsign: &call, OriginCountry: "Germany"},
			{ICAO24: "bbb222", OriginCountry: "France"},
		},
	}
}

// F3/resilience: when OpenSky fails, the page still renders (200) using the
// embedded sample data and clearly labels itself as sample.
func TestHandler_FallbackToSampleWhenOpenSkyFails(t *testing.T) {
	use(fakeFetcher{err: errors.New("opensky down")})

	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "badge sample") {
		t.Errorf("expected the sample-data badge in the page")
	}
	if strings.Contains(body, "badge live") {
		t.Errorf("did not expect the live badge when OpenSky failed")
	}
	// The sample snapshot has a fixed, non-empty set of aircraft.
	if n := len(opensky.SampleSnapshot().States); !strings.Contains(body, "aircraft tracked") || n == 0 {
		t.Errorf("sample snapshot should render some aircraft (have %d)", n)
	}
}

// When OpenSky responds, the page renders the live data and the live badge.
func TestHandler_LiveData(t *testing.T) {
	use(fakeFetcher{snap: liveSnapshot()})

	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "badge live") {
		t.Errorf("expected the live badge for a successful fetch")
	}
	if !strings.Contains(body, "Germany") {
		t.Errorf("expected the fetched data (Germany) in the page")
	}
}

// The JSON endpoint returns the decoded snapshot as valid JSON.
func TestHandler_JSON(t *testing.T) {
	use(fakeFetcher{snap: liveSnapshot()})

	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/?format=json", nil))

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
	var got opensky.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(got.States) != 2 {
		t.Errorf("decoded %d states, want 2", len(got.States))
	}
}

func TestBBoxFromQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?lamin=7&lomin=-92&lamax=18&lomax=-77", nil)
	if b := bboxFromQuery(r); !b.Set || b.LaMin != 7 || b.LoMax != -77 {
		t.Errorf("box = %+v, want set with 7/-77 bounds", b)
	}

	r = httptest.NewRequest(http.MethodGet, "/?lamin=7&lomin=-92", nil) // incomplete
	if bboxFromQuery(r).Set {
		t.Errorf("expected unset box when not all four params are present")
	}
}
