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

type viewCountry struct {
	Country string
	Count   int
	Pct     int // width of the bar, relative to the most frequent country
}

type pageData struct {
	Generated string
	Total     int
	Scope     string
	Live      bool
	Countries []viewCountry
	Sample    []aircraftRow
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
	max := 1
	if len(all) > 0 {
		max = all[0].Count // list is sorted descending
	}
	for _, c := range all {
		data.Countries = append(data.Countries, viewCountry{
			Country: c.Country,
			Count:   c.Count,
			Pct:     c.Count * 100 / max,
		})
	}

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
  :root {
    color-scheme: light dark;
    --bg: #f6f8fb; --panel: #ffffff; --ink: #0f172a; --muted: #64748b;
    --line: #e2e8f0; --accent: #0ea5e9; --accent-soft: #e0f2fe;
    --bar: #7dd3fc; --ok: #16a34a; --ok-soft: #dcfce7;
    --warn-bg: #fef3c7; --warn-ink: #92540b; --warn-line: #fcd34d;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #0b1220; --panel: #111a2e; --ink: #e8eef7; --muted: #94a3b8;
      --line: #223049; --accent: #38bdf8; --accent-soft: #0b2a3f;
      --bar: #0e7aa8; --ok: #4ade80; --ok-soft: #10331f;
      --warn-bg: #3a2e08; --warn-ink: #fcd34d; --warn-line: #7a5e0a;
    }
  }
  * { box-sizing: border-box; }
  body {
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    background: var(--bg); color: var(--ink); line-height: 1.5;
    margin: 0; padding: 2.5rem 1rem 4rem;
  }
  .wrap { max-width: 940px; margin: 0 auto; }
  header { display: flex; align-items: center; gap: .7rem; flex-wrap: wrap; }
  .logo { width: 34px; height: 34px; flex: 0 0 auto; }
  h1 { font-size: 1.7rem; margin: 0; letter-spacing: -.02em; }
  .badge { font-size: .72rem; font-weight: 700; letter-spacing: .04em;
           padding: .22rem .55rem; border-radius: 999px; text-transform: uppercase; }
  .badge.live { background: var(--ok-soft); color: var(--ok); }
  .badge.sample { background: var(--warn-bg); color: var(--warn-ink); }
  .meta { color: var(--muted); margin: .5rem 0 0; font-size: .88rem; }
  .banner {
    background: var(--warn-bg); color: var(--warn-ink);
    border: 1px solid var(--warn-line); border-radius: 12px;
    padding: .85rem 1.1rem; font-size: .9rem; margin: 1.3rem 0 0;
  }
  .banner code { background: #0002; padding: .05rem .35rem; border-radius: 5px; }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
           gap: 1rem; margin: 1.6rem 0 .5rem; }
  .card { background: var(--panel); border: 1px solid var(--line);
          border-radius: 14px; padding: 1.1rem 1.25rem; }
  .card .n { font-size: 2.1rem; font-weight: 750; letter-spacing: -.03em;
             line-height: 1; font-variant-numeric: tabular-nums; }
  .card .l { color: var(--muted); font-size: .82rem; margin-top: .4rem; }
  h2 { margin: 2.4rem 0 .8rem; font-size: 1.05rem; letter-spacing: -.01em; }
  .panel { background: var(--panel); border: 1px solid var(--line);
           border-radius: 14px; overflow: hidden; }
  table { border-collapse: collapse; width: 100%; font-size: .9rem; }
  th, td { text-align: left; padding: .6rem .9rem; border-bottom: 1px solid var(--line); }
  tbody tr:last-child td { border-bottom: 0; }
  th { color: var(--muted); font-weight: 600; font-size: .78rem;
       text-transform: uppercase; letter-spacing: .03em; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  td.mono { font-family: ui-monospace, "SF Mono", Menlo, monospace; font-size: .85rem; }
  .cc { display: flex; align-items: center; gap: .7rem; }
  .bar { flex: 1; height: 7px; border-radius: 999px; background: var(--accent-soft); overflow: hidden; }
  .bar > i { display: block; height: 100%; background: var(--bar); border-radius: 999px; }
  .cn { min-width: 2ch; text-align: right; font-variant-numeric: tabular-nums; font-weight: 600; }
  footer { margin-top: 2.6rem; color: var(--muted); font-size: .84rem; }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
  @media (max-width: 560px) { .hide-sm { display: none; } }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <svg class="logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"
         stroke-linecap="round" stroke-linejoin="round" style="color: var(--accent)">
      <path d="M21 16v-2l-8-5V3.5a1.5 1.5 0 0 0-3 0V9l-8 5v2l8-2.5V19l-2 1.5V22l3.5-1 3.5 1v-1.5L13 19v-5.5z"/>
    </svg>
    <h1>SkyPulse</h1>
    {{if .Live}}<span class="badge live">● Live</span>{{else}}<span class="badge sample">● Sample data</span>{{end}}
  </header>
  <p class="meta">Aircraft states from the OpenSky Network · snapshot {{.Generated}} · scope: {{.Scope}}</p>

  {{if not .Live}}<div class="banner">
    <strong>Showing embedded sample data.</strong> The live OpenSky API could not be reached
    from this server (OpenSky rate-limits and blocks datacenter IPs such as this host's).
    The data pipeline and rendering are identical, only the source differs.
    <br>Run it locally to see the live feed: <code>go run ./cmd/server</code> then open
    <code>localhost:8080</code> — OpenSky serves residential IPs normally.
  </div>{{end}}

  <div class="cards">
    <div class="card"><div class="n">{{.Total}}</div><div class="l">aircraft tracked</div></div>
    <div class="card"><div class="n">{{len .Countries}}</div><div class="l">top countries shown</div></div>
    <div class="card"><div class="n">{{if .Live}}Live{{else}}Sample{{end}}</div><div class="l">data source</div></div>
  </div>

  <h2>Aircraft by origin country</h2>
  <div class="panel"><table>
    <thead><tr><th>Country</th><th>Aircraft</th></tr></thead>
    <tbody>
    {{range .Countries}}<tr>
      <td>{{.Country}}</td>
      <td><div class="cc"><span class="bar"><i style="width:{{.Pct}}%"></i></span><span class="cn">{{.Count}}</span></div></td>
    </tr>
    {{else}}<tr><td colspan="2">No data.</td></tr>{{end}}
    </tbody>
  </table></div>

  <h2>Sample of tracked aircraft (first {{len .Sample}})</h2>
  <div class="panel"><table>
    <thead><tr>
      <th>ICAO24</th><th>Callsign</th><th>Country</th>
      <th class="hide-sm">Position</th><th class="num">Altitude</th><th class="num">Speed</th>
    </tr></thead>
    <tbody>
    {{range .Sample}}<tr>
      <td class="mono">{{.ICAO24}}</td><td>{{.Callsign}}</td><td>{{.Country}}</td>
      <td class="mono hide-sm">{{.Position}}</td><td class="num">{{.Altitude}}</td><td class="num">{{.Velocity}}</td>
    </tr>
    {{else}}<tr><td colspan="6">No data.</td></tr>{{end}}
    </tbody>
  </table></div>

  <footer>
    SkyPulse Data Collector ·
    <a href="?format=json">JSON</a> ·
    <a href="https://github.com/abrahamperz/skypulse-data-collector">source</a> ·
    data © <a href="https://opensky-network.org/">OpenSky Network</a>
  </footer>
</div>
</body>
</html>`))
