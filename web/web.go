// Package web renders a live view of a SkyPulse snapshot for the hosted demo.
// It turns an opensky.Snapshot into an HTML page or a JSON payload. It holds
// no state and touches no database, so it is safe to run in a serverless
// (stateless) environment such as Vercel.
package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"time"

	"arch3/opensky"
)

// CountryCount is the number of aircraft from one origin country in a snapshot.
type CountryCount struct {
	Country string
	Count   int
}

// CountByCountry groups states by origin country, most frequent first
// (ties broken alphabetically for a stable order).
func CountByCountry(states []opensky.State) []CountryCount {
	counts := map[string]int{}
	for _, s := range states {
		c := s.OriginCountry
		if c == "" {
			c = "(unknown)"
		}
		counts[c]++
	}
	out := make([]CountryCount, 0, len(counts))
	for c, n := range counts {
		out = append(out, CountryCount{Country: c, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Country < out[j].Country
	})
	return out
}

// RenderJSON writes the snapshot as indented JSON.
func RenderJSON(w http.ResponseWriter, snap *opensky.Snapshot) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snap)
}

type aircraftRow struct {
	ICAO24, Callsign, Country, Position, Altitude, Velocity string
}

type pageData struct {
	Generated    string
	Total        int
	Scope        string
	Live         bool
	TopCountries []CountryCount
	Sample       []aircraftRow
}

const sampleSize = 25
const topCountries = 15

// RenderHTML writes the snapshot as a small HTML dashboard. When live is false
// the data is the embedded sample (OpenSky was unreachable) and the page says so.
func RenderHTML(w http.ResponseWriter, snap *opensky.Snapshot, scope string, live bool) {
	data := pageData{
		Generated: time.Unix(snap.Time, 0).UTC().Format("2006-01-02 15:04:05 UTC"),
		Total:     len(snap.States),
		Scope:     scope,
		Live:      live,
	}

	all := CountByCountry(snap.States)
	if len(all) > topCountries {
		all = all[:topCountries]
	}
	data.TopCountries = all

	for i, s := range snap.States {
		if i >= sampleSize {
			break
		}
		data.Sample = append(data.Sample, aircraftRow{
			ICAO24:   s.ICAO24,
			Callsign: strOrDash(s.Callsign),
			Country:  s.OriginCountry,
			Position: position(s.Latitude, s.Longitude),
			Altitude: meters(s.GeoAltitude),
			Velocity: metersPerSec(s.Velocity),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

func strOrDash(p *string) string {
	if p == nil || *p == "" {
		return "—"
	}
	return *p
}

func position(lat, lon *float64) string {
	if lat == nil || lon == nil {
		return "—"
	}
	return fmt.Sprintf("%.3f, %.3f", *lat, *lon)
}

func meters(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f m", *p)
}

func metersPerSec(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f m/s", *p)
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SkyPulse — live aircraft</title>
<style>
  :root { color-scheme: light dark; }
  body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
         max-width: 920px; margin: 2rem auto; padding: 0 1rem; line-height: 1.5; }
  h1 { margin-bottom: .2rem; }
  .meta { color: #888; margin-top: 0; font-size: .9rem; }
  .cards { display: flex; gap: 1rem; flex-wrap: wrap; margin: 1.5rem 0; }
  .card { border: 1px solid #8884; border-radius: 10px; padding: 1rem 1.25rem; }
  .card .n { font-size: 2rem; font-weight: 700; }
  .card .l { color: #888; font-size: .85rem; }
  h2 { margin-top: 2rem; font-size: 1.1rem; }
  table { border-collapse: collapse; width: 100%; font-size: .9rem; }
  th, td { text-align: left; padding: .4rem .6rem; border-bottom: 1px solid #8883; }
  th { color: #888; font-weight: 600; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  .banner { background: #fbe8a6; color: #5c4400; border-radius: 8px;
            padding: .7rem 1rem; font-size: .88rem; }
  footer { margin-top: 2.5rem; color: #888; font-size: .85rem; }
  a { color: inherit; }
</style>
</head>
<body>
  <h1>SkyPulse</h1>
  <p class="meta">Aircraft states from the OpenSky Network · snapshot time {{.Generated}} · scope: {{.Scope}}</p>
  {{if not .Live}}<p class="banner">⚠ Showing embedded sample data: the live OpenSky API could not be reached from this server right now (it rate-limits and sometimes blocks datacenter IPs). The data pipeline and rendering are the same; only the source differs. Reload to retry the live feed.</p>{{end}}

  <div class="cards">
    <div class="card"><div class="n">{{.Total}}</div><div class="l">aircraft tracked</div></div>
    <div class="card"><div class="n">{{len .TopCountries}}</div><div class="l">top countries shown</div></div>
  </div>

  <h2>Aircraft by origin country (top {{len .TopCountries}})</h2>
  <table>
    <thead><tr><th>Country</th><th>Aircraft</th></tr></thead>
    <tbody>
    {{range .TopCountries}}<tr><td>{{.Country}}</td><td class="num">{{.Count}}</td></tr>
    {{else}}<tr><td colspan="2">No data.</td></tr>{{end}}
    </tbody>
  </table>

  <h2>Sample of tracked aircraft (first {{len .Sample}})</h2>
  <table>
    <thead><tr><th>ICAO24</th><th>Callsign</th><th>Country</th><th>Position (lat, lon)</th><th>Altitude</th><th>Speed</th></tr></thead>
    <tbody>
    {{range .Sample}}<tr><td>{{.ICAO24}}</td><td>{{.Callsign}}</td><td>{{.Country}}</td><td>{{.Position}}</td><td class="num">{{.Altitude}}</td><td class="num">{{.Velocity}}</td></tr>
    {{else}}<tr><td colspan="6">No data.</td></tr>{{end}}
    </tbody>
  </table>

  <footer>
    SkyPulse Data Collector · <a href="?format=json">JSON</a> ·
    data © <a href="https://opensky-network.org/">OpenSky Network</a>
  </footer>
</body>
</html>`))
