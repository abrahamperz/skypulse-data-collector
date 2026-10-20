package opensky

import "time"

// SampleSnapshot returns a small, fixed set of aircraft states. The hosted
// demo falls back to this when the live OpenSky API cannot be reached (OpenSky
// rate-limits and sometimes blocks datacenter IP ranges such as Vercel's), so
// the page always renders something that shows the application working.
func SampleSnapshot() *Snapshot {
	now := time.Now().Unix()
	s := func(v string) *string { return &v }
	f := func(v float64) *float64 { return &v }

	states := []State{
		{ICAO24: "3c6750", Callsign: s("DLH400"), OriginCountry: "Germany", Latitude: f(50.11), Longitude: f(8.68), GeoAltitude: f(11277), Velocity: f(242)},
		{ICAO24: "406b9a", Callsign: s("BAW117"), OriginCountry: "United Kingdom", Latitude: f(51.47), Longitude: f(-0.45), GeoAltitude: f(10668), Velocity: f(236)},
		{ICAO24: "a1b2c3", Callsign: s("UAL88"), OriginCountry: "United States", Latitude: f(40.64), Longitude: f(-73.78), GeoAltitude: f(9753), Velocity: f(221)},
		{ICAO24: "440123", Callsign: s("AFR22"), OriginCountry: "France", Latitude: f(48.86), Longitude: f(2.35), GeoAltitude: f(11582), Velocity: f(248)},
		{ICAO24: "0d0a11", Callsign: s("AMX7"), OriginCountry: "Mexico", Latitude: f(19.44), Longitude: f(-99.07), GeoAltitude: f(8534), Velocity: f(205)},
		{ICAO24: "e80401", Callsign: s("AVA92"), OriginCountry: "Colombia", Latitude: f(4.70), Longitude: f(-74.15), GeoAltitude: f(10363), Velocity: f(230)},
		{ICAO24: "7c6b2d", Callsign: s("QFA12"), OriginCountry: "Australia", Latitude: f(-33.95), Longitude: f(151.18), GeoAltitude: f(11887), Velocity: f(251)},
		{ICAO24: "c078ac", Callsign: s("ACA33"), OriginCountry: "Canada", Latitude: f(43.68), Longitude: f(-79.63), GeoAltitude: f(9144), Velocity: f(214)},
		{ICAO24: "3c4ab1", Callsign: s("DLH9"), OriginCountry: "Germany", Latitude: f(48.35), Longitude: f(11.78), OnGround: true},
		{ICAO24: "4ca7b4", Callsign: s("RYR18"), OriginCountry: "Ireland", Latitude: f(53.42), Longitude: f(-6.27), GeoAltitude: f(10972), Velocity: f(233)},
		{ICAO24: "471f2e", Callsign: s("WZZ5"), OriginCountry: "Hungary", Latitude: f(47.44), Longitude: f(19.26), GeoAltitude: f(11278), Velocity: f(240)},
		{ICAO24: "8a01f6", Callsign: s("UAE2"), OriginCountry: "United Arab Emirates", Latitude: f(25.25), Longitude: f(55.36), GeoAltitude: f(12192), Velocity: f(256)},
	}

	return &Snapshot{Time: now, FetchedAt: now, States: states}
}
