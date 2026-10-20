package opensky

import (
	"encoding/json"
	"testing"
)

// row builds a positional OpenSky state array from raw JSON literals.
func row(fields ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(fields))
	for i, f := range fields {
		out[i] = json.RawMessage(f)
	}
	return out
}

// fullRow is a well-formed 17-field state with a couple of null values,
// mirroring what OpenSky actually returns.
func fullRow() []json.RawMessage {
	return row(
		`"abc123"`,    // 0  icao24
		`"  DLH123 "`, // 1  callsign (padded, should be trimmed)
		`"Germany"`,   // 2  origin_country
		`1700000000`,  // 3  time_position
		`1700000001`,  // 4  last_contact
		`8.5`,         // 5  longitude
		`50.1`,        // 6  latitude
		`null`,        // 7  baro_altitude (unknown)
		`false`,       // 8  on_ground
		`230.4`,       // 9  velocity
		`180.0`,       // 10 true_track
		`-2.3`,        // 11 vertical_rate
		`null`,        // 12 sensors (ignored)
		`11000`,       // 13 geo_altitude
		`"1000"`,      // 14 squawk
		`false`,       // 15 spi
		`0`,           // 16 position_source
	)
}

// F2: a well-formed row decodes into typed fields, nulls stay nil, callsign trimmed.
func TestDecodeState_WellFormed(t *testing.T) {
	s, err := decodeState(fullRow())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ICAO24 != "abc123" {
		t.Errorf("icao24 = %q, want abc123", s.ICAO24)
	}
	if s.Callsign == nil || *s.Callsign != "DLH123" {
		t.Errorf("callsign = %v, want trimmed DLH123", s.Callsign)
	}
	if s.OriginCountry != "Germany" {
		t.Errorf("origin_country = %q, want Germany", s.OriginCountry)
	}
	if s.BaroAltitude != nil {
		t.Errorf("baro_altitude = %v, want nil (null preserved as unknown)", *s.BaroAltitude)
	}
	if s.Longitude == nil || *s.Longitude != 8.5 {
		t.Errorf("longitude = %v, want 8.5", s.Longitude)
	}
	if s.OnGround {
		t.Errorf("on_ground = true, want false")
	}
}

// F3: a short / malformed row is rejected so the caller can skip it.
func TestDecodeState_ShortRow(t *testing.T) {
	if _, err := decodeState(row(`"abc123"`, `"CALL"`)); err == nil {
		t.Fatalf("expected error for a row with too few fields, got nil")
	}
}

// F7: the bounding box is only active when all four variables are valid.
func TestBBoxFromEnv(t *testing.T) {
	t.Run("all four set", func(t *testing.T) {
		t.Setenv("OPENSKY_LAMIN", "7")
		t.Setenv("OPENSKY_LOMIN", "-92")
		t.Setenv("OPENSKY_LAMAX", "18")
		t.Setenv("OPENSKY_LOMAX", "-77")
		b := BBoxFromEnv()
		if !b.Set {
			t.Fatalf("Set = false, want true when all four vars are present")
		}
		if b.LaMin != 7 || b.LoMin != -92 || b.LaMax != 18 || b.LoMax != -77 {
			t.Errorf("box = %+v, want 7/-92/18/-77", b)
		}
	})

	t.Run("one missing", func(t *testing.T) {
		t.Setenv("OPENSKY_LAMIN", "7")
		t.Setenv("OPENSKY_LOMIN", "-92")
		t.Setenv("OPENSKY_LAMAX", "18")
		// OPENSKY_LOMAX intentionally left unset.
		if BBoxFromEnv().Set {
			t.Fatalf("Set = true, want false when a variable is missing")
		}
	})
}
