// Package opensky is a small client for the OpenSky Network public REST API.
// It fetches the /states/all endpoint and decodes each aircraft state vector,
// which OpenSky returns as a heterogeneous JSON array rather than an object.
package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const statesURL = "https://opensky-network.org/api/states/all"

// Client fetches live aircraft states from the OpenSky Network REST API.
type Client struct {
	http *http.Client
}

// New returns a Client whose requests use the given timeout.
func New(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

// BBox is an optional geographic bounding box used to limit the query to a region.
type BBox struct {
	LaMin, LoMin, LaMax, LoMax float64
	Set                        bool
}

// BBoxFromEnv builds a BBox from OPENSKY_LAMIN / OPENSKY_LOMIN /
// OPENSKY_LAMAX / OPENSKY_LOMAX. All four must be present and valid,
// otherwise an unset BBox is returned and the whole world is queried.
func BBoxFromEnv() BBox {
	get := func(k string) (float64, bool) {
		v := os.Getenv(k)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	laMin, ok1 := get("OPENSKY_LAMIN")
	loMin, ok2 := get("OPENSKY_LOMIN")
	laMax, ok3 := get("OPENSKY_LAMAX")
	loMax, ok4 := get("OPENSKY_LOMAX")
	if ok1 && ok2 && ok3 && ok4 {
		return BBox{LaMin: laMin, LoMin: loMin, LaMax: laMax, LoMax: loMax, Set: true}
	}
	return BBox{}
}

// State is one aircraft's decoded state vector. Pointer fields are nullable
// because OpenSky reports null for values it does not currently have.
type State struct {
	ICAO24         string   // unique ICAO 24-bit transponder address
	Callsign       *string  // flight callsign, trimmed
	OriginCountry  string   // country inferred from the transponder address
	TimePosition   *int64   // unix time of the last position report
	LastContact    *int64   // unix time of the last message received
	Longitude      *float64 // WGS-84 longitude in decimal degrees
	Latitude       *float64 // WGS-84 latitude in decimal degrees
	BaroAltitude   *float64 // barometric altitude in meters
	OnGround       bool     // true when the aircraft is on the ground
	Velocity       *float64 // speed over ground in m/s
	TrueTrack      *float64 // heading in decimal degrees (0 = north)
	VerticalRate   *float64 // vertical rate in m/s (positive = climbing)
	GeoAltitude    *float64 // geometric altitude in meters
	Squawk         *string  // transponder code
	SPI            bool     // special purpose indicator
	PositionSource *int64   // origin of the position (ADS-B, MLAT, etc.)
}

// Snapshot is a single OpenSky response: a timestamp plus every aircraft
// state at that moment, and the local time we fetched it.
type Snapshot struct {
	Time      int64 // "time" reported by OpenSky (unix seconds)
	FetchedAt int64 // local unix time when the collector made the request
	States    []State
}

type rawResponse struct {
	Time   int64               `json:"time"`
	States [][]json.RawMessage `json:"states"`
}

// FetchStates calls GET /states/all and returns a decoded Snapshot.
func (c *Client) FetchStates(ctx context.Context, bbox BBox) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statesURL, nil)
	if err != nil {
		return nil, err
	}
	if bbox.Set {
		q := req.URL.Query()
		q.Set("lamin", strconv.FormatFloat(bbox.LaMin, 'f', -1, 64))
		q.Set("lomin", strconv.FormatFloat(bbox.LoMin, 'f', -1, 64))
		q.Set("lamax", strconv.FormatFloat(bbox.LaMax, 'f', -1, 64))
		q.Set("lomax", strconv.FormatFloat(bbox.LoMax, 'f', -1, 64))
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("User-Agent", "SkyPulse-DataCollector/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensky returned HTTP %d", resp.StatusCode)
	}

	var raw rawResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	snap := &Snapshot{Time: raw.Time, FetchedAt: time.Now().Unix()}
	for _, row := range raw.States {
		s, err := decodeState(row)
		if err != nil {
			// Skip a malformed row rather than failing the whole batch.
			continue
		}
		snap.States = append(snap.States, s)
	}
	return snap, nil
}

// decodeState maps one positional OpenSky state array into a State.
// Field order is defined by the OpenSky API documentation.
func decodeState(row []json.RawMessage) (State, error) {
	var s State
	if len(row) < 17 {
		return s, fmt.Errorf("state row has %d fields, expected at least 17", len(row))
	}
	if err := json.Unmarshal(row[0], &s.ICAO24); err != nil {
		return s, fmt.Errorf("icao24: %w", err)
	}
	s.Callsign = optString(row[1])
	if s.Callsign != nil {
		trimmed := strings.TrimSpace(*s.Callsign)
		s.Callsign = &trimmed
	}
	_ = json.Unmarshal(row[2], &s.OriginCountry)
	s.TimePosition = optInt(row[3])
	s.LastContact = optInt(row[4])
	s.Longitude = optFloat(row[5])
	s.Latitude = optFloat(row[6])
	s.BaroAltitude = optFloat(row[7])
	if v := optBool(row[8]); v != nil {
		s.OnGround = *v
	}
	s.Velocity = optFloat(row[9])
	s.TrueTrack = optFloat(row[10])
	s.VerticalRate = optFloat(row[11])
	// row[12] is the sensors array, which we do not store.
	s.GeoAltitude = optFloat(row[13])
	s.Squawk = optString(row[14])
	if v := optBool(row[15]); v != nil {
		s.SPI = *v
	}
	s.PositionSource = optInt(row[16])
	return s, nil
}

func isNull(r json.RawMessage) bool {
	return len(r) == 0 || string(r) == "null"
}

func optString(r json.RawMessage) *string {
	if isNull(r) {
		return nil
	}
	var v string
	if err := json.Unmarshal(r, &v); err != nil {
		return nil
	}
	return &v
}

func optFloat(r json.RawMessage) *float64 {
	if isNull(r) {
		return nil
	}
	var v float64
	if err := json.Unmarshal(r, &v); err != nil {
		return nil
	}
	return &v
}

func optInt(r json.RawMessage) *int64 {
	if isNull(r) {
		return nil
	}
	var v int64
	if err := json.Unmarshal(r, &v); err == nil {
		return &v
	}
	// Some numeric fields may arrive as floats (for example 1.0).
	var f float64
	if err := json.Unmarshal(r, &f); err == nil {
		iv := int64(f)
		return &iv
	}
	return nil
}

func optBool(r json.RawMessage) *bool {
	if isNull(r) {
		return nil
	}
	var v bool
	if err := json.Unmarshal(r, &v); err != nil {
		return nil
	}
	return &v
}
