package web

import (
	"testing"

	"arch3/internal/opensky"
)

func TestCountByCountry(t *testing.T) {
	states := []opensky.State{
		{OriginCountry: "Germany"},
		{OriginCountry: "France"},
		{OriginCountry: "Germany"},
		{OriginCountry: "Germany"},
		{OriginCountry: "France"},
		{OriginCountry: ""}, // becomes "(unknown)"
	}

	got := CountByCountry(states)

	if len(got) != 3 {
		t.Fatalf("got %d groups, want 3", len(got))
	}
	// Most frequent first.
	if got[0].Country != "Germany" || got[0].Count != 3 {
		t.Errorf("first = %+v, want Germany/3", got[0])
	}
	if got[1].Country != "France" || got[1].Count != 2 {
		t.Errorf("second = %+v, want France/2", got[1])
	}
	if got[2].Country != "(unknown)" || got[2].Count != 1 {
		t.Errorf("third = %+v, want (unknown)/1", got[2])
	}
}
