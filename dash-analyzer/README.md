# Melbourne delivery optimizer

A Next.js + Go planning tool with a Leaflet map for Melbourne car delivery drivers. Uses OpenStreetMap
food and grocery venues, Open-Meteo weather and your own SQLite earnings ledger. No API keys,
paid services or DoorDash integration. The optional standalone Maps collector below
uses browser extraction; the dashboard itself uses free public APIs. Scores are **heuristics, never live
DoorDash demand, verified platform participation, order predictions or promised income**.

## Run

Install **Go 1.25+ and Node.js 24+** and pnpm 12+, then:

```sh
make run
# New Next.js dashboard: http://127.0.0.1:3000
# Go API: http://127.0.0.1:8080/api/health
```

`make run` installs the locked frontend dependencies when needed and starts both
services. Stop an older Go server first if it already occupies port 8080. Ctrl+C
stops both processes. `FRONTEND_PORT` changes the Next.js port; `PORT` changes the
Go port. Preferences are saved in this browser; delivery records remain in local
SQLite, with no migration or upload required.

For separate terminals, use `make api` and `make frontend`. If reusing an already
running Go service, run only `make frontend`. Set `GO_API_URL` on the Next.js process
when Go is at a different address (default `http://127.0.0.1:8080`). This variable
is server-only; browser requests use the same-origin `/api` proxy. The legacy
Leaflet page is still available directly from Go at `/web/`.

Or use `docker compose up --build -d` and open **http://127.0.0.1:8080**. Compose now
runs separate Go and Next.js services; the frontend proxies API requests over the
internal Docker network. Only the frontend is published, on localhost. The existing
`analyzer-data` volume still holds the SQLite ledger and venue cache. `docker compose
down` retains it; deleting the volume deletes your history. Local native files live
in `data/`. Back up that directory with the server stopped (SQLite WAL files matter
while running). Node and Chromium are included in their respective Docker images.

Configuration: `DATA_DIR` (default `data`), `PORT` (default `8080`) or `LISTEN_ADDR`
(default `127.0.0.1:8080`). There is no authentication: keep this personal tool on
localhost. A reverse proxy with authentication is needed before sharing it.

Startup serves the dashboard immediately and refreshes OSM in the background.
Missing data produces empty rankings, never sample restaurants. A failed refresh
retains the existing cache. Venue cache TTL is 24 hours, checked hourly; manual
refresh has a one-minute successful-refresh cooldown. Collection makes up to three
attempts with exponential backoff and endpoint failover, bounded by a 130-second
context. Valid non-empty results replace cache atomically. Weather is cached for
15 minutes with one-minute failure backoff; stale weather is shown and excluded
from scoring. A network connection is required for initial data, map tiles and
Leaflet assets. Weather uses the free Open-Meteo endpoint intended for noncommercial
use; check provider terms before commercial deployment.

## Using it

1. Enter your location or use browser geolocation (localhost or HTTPS), consumption
   in L/100km, fuel price in AUD/litre, and parking sensitivity.
2. Update recommendations and inspect clusters on the map. Use Minimum cluster
   score to show all clusters or only scores of 20, 40, 60 or 80 and above. This
   instantly filters both map clusters and the top 20 sidebar results; individual
   restaurant markers remain controlled separately. The top five are also
   available from `/api/recommendations`. Consider staying nearby when relocation
   costs outweigh the uncertain benefit. Scores alone cannot prove moving pays.
3. Log completed deliveries or non-overlapping work blocks. Include waiting,
   repositioning, return kilometres and all applicable costs. Fuel cost is entered
   explicitly in the ledger, so historical costs do not change with current settings.
   Read history using the API; the dashboard shows the newest 20 entries.

Net = gross − fuel − other costs. Net/hour divides total net by summed logged hours;
net/km divides by total logged km. Zero denominators return zero. These are realized
statistics for the entries you supply, not forecasts. Overlapping entries double
count time; omitted waiting time inflates hourly results. Include maintenance,
insurance allocation, parking and tolls under other costs if relevant. No tax model.

## Scoring and coverage

Coverage is a fixed Melbourne metropolitan bounding box: latitude −38.05 to −37.55,
longitude 144.65 to 145.35. Locations outside it are rejected. Restaurants, fast food,
cafes and food courts are collected as OSM nodes, ways and relations with centres.
OSM type/id duplicates are removed; distinct objects describing the same business
may remain. Venue caches retain name, coordinates, amenity type (the API
`type` field), cuisine, takeaway and opening_hours. Existing caches without new
fields work with unknown hours; refresh OSM to populate them. Platform
participation is unknown.

Clusters greedily group unassigned venues within 800m of a geographically sorted
seed and display the member centroid; every venue belongs to one cluster. This is
radius clustering, not DBSCAN; its boundaries are approximate. Fast food weight is
1.4; grocery shops (supermarket, convenience, grocery) 2.0; other venues 1.0,
with takeaway yes/only ×1.2. Grocery weighting is a planning preference, not
evidence of higher demand or earnings.

Activity uses Melbourne local calendar time with DST. These relative factors are
uncalibrated assumptions, not Google Maps popularity, live demand or measured orders:

| Venue | Breakfast 06–10 | Lunch 11–14 | Dinner 17–21 | Late night 22–02 | Other hours |
| --- | --- | --- | --- | --- | --- |
| Restaurant/default | 0.35 | 1.15 | 1.4 | 0.3 | 0.2 |
| Cafe | 1.3 | 1.1 | 0.25 | 0.05 | 0.3 |
| Fast food | 0.55 | 1.2 | 1.3 | 0.8 | 0.3 |
| Food court | 0.3 | 1.3 | 1.0 | 0.1 | 0.2 |

Cuisine modifiers: breakfast/coffee_shop ×1.25 at breakfast; sandwich/salad ×1.15
at lunch; pizza/indian/thai/chinese ×1.15 at dinner; burger/kebab ×1.25 late night.
Semicolon-separated matching cuisines each apply their modifier. Saturday and
Sunday multiply all factors by 1.1 and breakfast by another 1.1; Friday dinner
and late night multiply by 1.1. Monday–Thursday use the base factors.

Opening hours use [chneau/openhours](https://github.com/chneau/openhours), with a
strict weekly-subset guard because the library is permissive. Supported inputs:
24/7, open/off/closed, weekday lists/ranges, clock intervals (including overnight),
multiple comma-separated intervals, and trailing off overrides. Repeated weekdays
in opening rules and reopening after off rules remain unknown because the parser
cannot safely represent OSM replacement semantics. Missing, malformed or unsupported inputs
(holiday/date/solar rules, comments, fallback syntax) remain unknown. Confirmed
open/closed means according to the selected cached source schedule, not verified
on site. The Google hours precedence and freshness rules are documented below.
Confirmed closed contributes zero. Unknown hours retain the category/cuisine
estimate with low confidence; supported schedules have medium confidence.
Confidence describes source coverage, not statistical accuracy. All seven days
are supported; these profiles do not model public holidays.

API clusters retain all venues for context and report activity_score (weighted
activity before parking/weather/travel), confirmed_open, confirmed_closed,
unknown_hours and confidence. Any unknown hours makes cluster confidence low.
All-closed clusters remain visible at score zero. Individual markers show status,
profile, factor and confidence. The legacy grid Analyze function also accepts a
selected datetime, reports these breakdowns and excludes empty active zones;
its raw_score includes parking and score is normalized to the highest grid score.

Score = `100 × (1 − exp(−timeWeightedDensity/12)) × (1 − parkingPenalty)
× weatherFactor / (1 + estimatedRoadKM/8 + fuelCostAUD/4)`.
Parking penalty is `0.55 × sensitivity × exp(−(CBDdistanceKM/2.5)^2)`.
Weather factor is `1 − min(0.3, precipitationMM×0.03 + windKMH×0.001)` when fresh;
otherwise 1. Weather reduces access suitability and does not imply order demand.
There is one Melbourne-wide weather observation, not street-level weather.

Road km = Haversine distance × road factor (default 1.3). Travel minutes assume
25km/h. Fuel AUD = road km × L/100km × AUD/litre / 100. These estimates do not use
road routing, traffic, tolls, restrictions or real parking availability. Scores use
absolute saturation rather than making the best cluster automatically score 100.
The original grid scorer remains internally available for comparison.

## REST API

JSON responses; validation failures return HTTP 400 with an `error` field.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | Process/database liveness (not upstream freshness) |
| GET | `/api/zones` | All ranked clusters, venues, weather and cache freshness |
| GET | `/api/recommendations` | Top five clusters, same metadata |
| POST | `/api/refresh` | Refresh food and grocery venues; 502 on upstream failure |
| GET | `/api/weather` | Cached observation with available/stale flags |
| GET | `/api/deliveries` | All logged entries, newest first |
| POST | `/api/deliveries` | Persist one entry, return HTTP 201 |
| GET | `/api/analytics` | Aggregate realized gross/net, hours and km |
| GET | `/api/maps/venues?lat=…&lon=…` | Cached OSM venues within 5 km with stored Maps observations |
| POST | `/api/maps/collect?lat=…&lon=…` | Start a background Maps collection; returns 202 |
| GET | `/api/maps/job` | Latest collection progress and per-venue results |
| DELETE | `/api/maps/job` | Cancel the running collection |

Zones/recommendations query parameters: `lat`, `lon`, `parking` (0–1),
`litres_per_100km` (0–40), `fuel_price` (0–10 AUD), `road_factor` (1–3).
Defaults: Melbourne CBD, 0.65, 8, 2, 1.3 respectively.
Optional `day` is an integer 0–6 (Sunday–Saturday); `hour` is an integer 0–23.
Omitting `date`, `day` and `hour` uses the current instant in Australia/Melbourne. With a day or hour
parameter, the omitted component defaults to the current local value; the day
selects its next occurrence including today, at minute zero. Dates use calendar
arithmetic across DST. A nonexistent spring-forward hour returns HTTP 400;
a repeated autumn hour uses Go's time.Date choice and the response includes its
offset. Invalid, empty, repeated day/hour and nonfinite numeric values are rejected.
Optional `date=YYYY-MM-DD` selects that exact Melbourne calendar date, including
past dates, instead of the next weekday occurrence. It cannot be combined with
`day`. The hour defaults to the current Melbourne hour when omitted; minutes are
zero. Invalid, empty or repeated date values return 400. DST validation and offset
reporting apply to explicit dates too.

Responses include `selected_local_datetime` and `scored_at` as RFC3339 timestamps
with Melbourne's offset, `timezone`, per-venue `activity`, and cluster breakdowns.
Weather remains the current cached observation, even for a future selected day.

Use the dashboard day selector and hour slider to recalculate estimates; the
parking slider, score filter and individual restaurant markers remain available.

```sh
curl 'http://localhost:8080/api/recommendations?lat=-37.81&lon=144.96&fuel_price=1.95'
curl -X POST http://localhost:8080/api/deliveries \
  -H 'Content-Type: application/json' \
  -d '{"started_at":"2026-11-10T17:00:00+11:00","ended_at":"2026-11-10T18:00:00+11:00","gross_aud":38,"distance_km":18,"fuel_cost_aud":2.88,"other_cost_aud":3,"notes":"Dinner work block"}'
```

Delivery timestamps must be RFC3339 with offsets, duration greater than zero and
at most 24 hours. Amounts/km must be nonnegative and at most 1,000,000. Notes are
limited to 2,000 bytes. Ledger listing currently has no pagination or edit/delete
UI. Monetary arithmetic uses float64; this is a personal analysis tool, not an
accounting system.

## Development

```sh
gofmt -w cmd internal
go test ./...
go build ./...
# optional concurrency checks
go test -race ./...
```

Unit and HTTP integration tests use in-process httptest recorders and local fixtures and temporary databases, with
no calls to public APIs. They cover retries, incomplete OSM responses, cancellation,
cache persistence, weather fallback, clustering, local-time/parking/weather effects,
fuel math, ledger analytics, validation and API persistence across reopening.
The Docker image can be verified with `docker build -t dash-analyzer:local .`;
the new frontend Docker image is built from `frontend/Dockerfile`. Browser QA uses
fixture API responses and blocks public map tiles, rather than writing to your ledger.

Data: [OpenStreetMap contributors](https://www.openstreetmap.org/copyright) (ODbL),
[Overpass API](https://wiki.openstreetmap.org/wiki/Overpass_API),
[Open-Meteo](https://open-meteo.com/en/docs) (weather attribution CC BY 4.0).
Public services may rate-limit or be unavailable. Leaflet and OSM tiles are loaded
from public servers; respect their usage policies. Not affiliated with DoorDash.

The opening-hours dependency is kept as an unmodified local source snapshot in
`third_party/openhours` with its MIT license and provenance, so builds require no
network download for this parser. Go's build cache must be writable; in restricted
environments use `GOCACHE=/tmp/dash-analyzer-go-cache` for tests and builds.

### Optional grocery shops

Enable **Include grocery shops** to add OSM `shop=supermarket`, `convenience`
and `grocery` venues to rankings and individual markers (purple for groceries).
The option defaults off; food venues remain included. Refresh OSM after upgrading
to populate grocery shops in an existing cache. Collection caches both categories;
failed refreshes preserve the previous cache. Grocery platform participation is unknown.

Both `/api/zones` and `/api/recommendations` accept `include_grocery=true|false`
(default false; invalid or repeated values return 400). Filtering happens before
clustering, so returned venues, counts and rankings use the same selection.
Grocery activity uses a neutral factor of 1 at all times and weekdays with profile
`grocery`, without meal, cuisine or day modifiers. This is an uncalibrated baseline,
not measured shopping activity. Supported OSM hours still set closed venues to zero;
missing or unsupported hours retain low confidence. Base cluster/grid grocery weight is 2 (twice the previous baseline of 1),
with the existing takeaway multiplier when tagged.

### Range and scoring settings

Open **Scoring settings** to set a range of 0–100 km from your entered location
(0 means all cached Melbourne coverage). The range uses Haversine straight-line
kilometres to each venue and filters before clustering, including markers and
counts. It does not expand OSM coverage. An orange outline shows the range and
an orange point shows the selected origin. Click **Select location on map**, then
click an empty map area to set the origin and recalculate. Selection ends after
one valid click; use Cancel or Escape to exit without moving the point. Ordinary
map clicks leave the origin unchanged; venue/cluster clicks open their details. Locations
outside the existing Melbourne bounds are rejected.

Enable **Ignore distance in scoring** to remove both the road-distance and fuel
terms from the score denominator (making it 1). This prevents relocation fuel cost
from indirectly penalizing distance. Estimated road km, travel minutes and fuel
cost remain visible; parking, activity and weather still affect rankings. The
range filter applies independently. Defaults retain the original distance scoring. The dashboard range now starts at
5 km; API `radius_km` still defaults to 0 when omitted.

Zones/recommendations accept `radius_km` (0–100, default 0) and
`ignore_distance=true|false` (default false), and echo both in responses.
Empty/repeated range values and invalid/repeated booleans return HTTP 400.

## Standalone Google Maps venue library

`pkg/mapsdata` and `cmd/venue-enrich` provide an **optional, explicit browser
collection job**. The Go scoring API uses its saved fresh opening-hour observations
while keeping OSM schedules and Google source records separate.
It collects **opening hours and available popular-times labels** from Google Maps
using Chromium via [chromedp](https://github.com/chromedp/chromedp). No paid API,
API key or logged-in browser profile is used. Chromium/Chrome must be installed
on the machine running this command. The Docker server image now includes Chromium.

Supply a JSON array of explicitly mapped venues. `key` is your stable venue key
(use OSM `osm_type/id` to associate a cached venue). `name` is the expected Google
place heading, compared ignoring case and whitespace. Copy a full place URL,
not a search or shortened link; the collector does not guess matches from names.
For example, replace the placeholders in `venues.json` with actual mappings:

```json
[
  {
    "key": "node/REPLACE_WITH_OSM_ID",
    "name": "REPLACE_WITH_GOOGLE_PLACE_HEADING",
    "maps_url": "https://www.google.com/maps/place/REPLACE_WITH_FULL_PLACE_URL"
  }
]
```

```sh
# Parent directory must exist. This can share the local delivery database.
go run ./cmd/venue-enrich -venues venues.json -db data/deliveries.db \
  -chromium /usr/bin/chromium -interval 5s -timeout 45s
```

The command prints JSON outcomes per venue and exits nonzero for failures or
partial results. It runs sequentially with a five-second gap by default (CLI
minimum one second), a per-venue timeout and cancellation on Ctrl+C. Consent,
CAPTCHA/challenge pages, mismatched place headings, empty results and unsupported
layouts are failures; it does not bypass challenges or sign in. Browser selectors
are best effort against an undocumented English Google Maps DOM and may need
maintenance. Only labels currently rendered on the page are collected; **a full
seven-day popular-times profile is not guaranteed**. An unidentified weekday stays
empty. Live/current-busyness labels are excluded. Opening hours retain displayed
weekday text verbatim, including split periods, closed days and special hours;
they are not converted into OSM rules or assumed to be a permanent weekly schedule.

`google_maps_venues` stores JSON busy labels and opening hours, expected identity,
source URL/title, separate successful component timestamps, and the latest attempt
and error. Missing components and failed collection **preserve previous data**;
retained observations keep their old timestamps. Changing an existing key's name
or URL is rejected; use a new key for a different mapping. Data stays in the local
SQLite file. Existing delivery tables and OSM caches are unaffected. The library stores observations; the main app applies the schedule checks described
below. The dashboard integration
runs collection only when requested; no periodic Maps collection is enabled.

To use the library directly, call `mapsdata.Open(dbPath)`, then
`mapsdata.Collector{Fetcher: mapsdata.Chromium{}, Store: store}.Collect(ctx, venues)`.
Use `store.Get(ctx, key)` to retrieve cached values with component timestamps and
last-error status, and `store.Close()` when finished. `Fetcher` is injectable for
tests or another collector. Missing busy data never means zero, and popular-times
labels do not measure DoorDash orders, participation, earnings or driver counts.
Google describes popular times as aggregated visitor activity, not delivery demand
([source](https://support.google.com/business/answer/6263531)).

Normal tests use in-process fixtures and temporary SQLite files. An optional
Chromium extraction smoke test also uses only an inline fixture, without a network
listener or public requests:

```sh
MAPSDATA_TEST_CHROMIUM=/usr/bin/chromium go test ./pkg/mapsdata -run TestChromiumExtractionFixture -v
```

Live Google Maps extraction is not part of the automated checks; fixture success
does not guarantee compatibility with every live page or layout.

### Collect within 5 km in the main app

Restart the server after upgrading (`make run`, or rebuild with
`docker compose up --build -d`). For a native server install Chromium/Chrome and,
if auto-detection fails, set `CHROMIUM_PATH=/usr/bin/chromium` before starting.
The server checks browser availability when a collection is requested and returns
503 with an installation message if missing. Container hosts must support the
Chromium sandbox; the app does not disable it.

1. Choose your origin with **Select location on map**, coordinates or geolocation.
2. Use **Refresh OSM Data** to populate/update the venue cache.
3. Open **Google Maps data within 5 km** and click **Collect within 5 km**.
4. Inspect progress and expand venue rows for cached busy-time labels, opening
   hours, source links and separate component timestamps. **View cached data**
   reloads saved results; **Cancel collection** stops the background job.

Collection includes **all food and grocery venues in the OSM cache within a
5 km straight-line radius**, regardless of the scoring range or grocery toggle.
It does not discover every business on Google Maps or expand the fixed Melbourne
coverage. Empty coverage returns 400 with a prompt to refresh OSM first. Origin
coordinates are required, finite, single-valued and inside the existing bounds.
The latest accepted job captures its origin and venue list: moving the map point
after starting does not change that job, and its progress shows the captured origin.

For venues without a stored mapping, the browser searches using name and OSM
coordinates, using a map viewport centred on the venue rather than putting the
coordinates into a text query. A result must expose actual place coordinates
within 250 metres for an exact name, or within 100 metres for a name followed by
a branch suffix (for example, OSM “McDonald's” and Google “McDonald's Northland”).
Case, whitespace, curly apostrophes and `&`/`and` spelling are normalized for
this matching step (for example, “Burger Daddy and Kebab” versus
“Burger Daddy & Kebab”). Coordinate and unique-result checks still apply.
Suffix matching requires a base name of at least five characters. There must be
exactly one matching place URL.
Map viewport coordinates are not accepted as evidence. Ambiguous, unnamed,
missing-coordinate or mismatched results are reported without saving another
business's observations. The tolerance accommodates approximate OSM centres; it
is a matching heuristic, not a guarantee. Name matching may still miss legitimate
businesses with different Google names. Existing explicit mappings from the CLI
are reused. Manual CLI mappings remain available for those cases.

One job runs at a time (a second start returns 409). The job runs independently of
its HTTP request, sequentially with a five-second gap, and reports processed,
partial and failed venues. Complete observations less than 24 hours old are reused.
Challenges stop the batch; other per-venue failures are reported and processing
continues. Partial/failed collection preserves cached components. Jobs and
unresolved-match errors are held in memory and do not resume after a server
restart; successful observations and mapped-place collection errors remain in
SQLite. Shutdown cancels the job and waits for Chromium before closing the database.

Google data is displayed in its own section. Fresh readable opening hours also
affect activity scoring; busy-time labels remain source observations. Collection remains best effort: Google may omit busy
charts, require consent, or change its layout. Full seven-day coverage is not
guaranteed, and no missing chart is interpreted as zero activity.

### Chromium protocol compatibility

The Maps collector pins chromedp v0.23.0 and cdproto v0.159.2, including support
for Chromium 152's `initialFrameNavigation` event. These dependencies require
Go 1.25 or newer. If an older running binary logs `unknown ClientNavigationReason
value: initialFrameNavigation`, restart with the rebuilt application (`make run`
or `docker compose up --build -d`). Updating source files alone does not update a
running process. A regression test decodes the exact navigation event; no Maps
requests are needed to verify the protocol fix.

### Collection logs

Maps collection logs are written to the server terminal (or `docker compose logs -f`)
and to stderr for `venue-enrich`, keeping its JSON stdout usable. Logs include
batch start/end, per-venue progress, Google matching, cache/mapping reuse, fetching,
SQLite save outcomes, cancellation, errors and elapsed milliseconds. Venue keys and
names identify each attempt; job summaries include processed, cached and issue
counts. Partial data and upstream failures use WARN; database saves and stopped
challenge batches use ERROR. No delivery ledger entries or full page contents are
logged. Logs are local; the app does not upload them or create a separate log file.

### Google limited-view pages

Google can return a page explicitly labelled “limited view of Google Maps” to the
collector. Such a page may display opening hours without any popular-times chart.
The collector waits for delayed place controls, opens the hours disclosure when
available, and reads explicit weekday rows even when the table is outside a
`role=main` panel. Extra copy-button text is excluded. Only days actually present
are collected: a single Tuesday “Open 24 hours” row is not expanded into a full
weekly schedule. Missing busy charts are reported with a limited-view availability
note in the partial result/last-error record; older busy data is preserved.

Browser diagnostics log limited-view detection, hours expansion, extracted day
counts, search result counts and verified matches. These identify missing data
and matching failures without logging the full page. The collector does not bypass
Google's limited view or imply that increasing a timeout can reveal missing charts.

### Navigation and hours panels

Google Maps can replace its JavaScript execution context during redirects or
navigation. Read-only DOM evaluations and readiness checks retry these specific
transient context errors up to three times with short backoff, within the original
per-venue timeout. Clicks are not replayed, since a second click can close a panel.
Other errors, including wrong-place identity, are not retried by this mechanism.

Opening hours can replace the visible place heading with “Hours” and hide the
busy chart. Collection retains the already verified name and any chart observed
before opening the panel, but only if the resulting URL still identifies the same
Google place ID (or identical place path when IDs are absent). Unrelated names
and changed place IDs remain failures. Limited-view status is retained when the
panel hides that notice. Different names such as “Rosanna Fish and Chips” versus
“Rosanna Fish & Chippery” still require an explicit mapping; the collector does not
use broad fuzzy name matching that could accept an unrelated business.

## Next.js frontend

The new app lives in `frontend/`, uses **Next.js App Router, React and strict
TypeScript**, and has four routes:

| Route | Page | Features |
| --- | --- | --- |
| `/` | Explore | Leaflet map, deliberate click-to-select origin, geolocation, venue markers, score filtering and ranked clusters |
| `/collection` | Venue intelligence | 5 km collection, captured-origin progress, cancellation and expandable cached source observations |
| `/history` | Delivery ledger | Local history and analytics, plus a form for completed work blocks |
| `/settings` | Scoring settings | Shared origin, range, meal time, parking, grocery weighting options, fuel and road-distance assumptions |

Settings persist under `dash-settings-v1` in browser localStorage and are shared
across routes. They do not change driver-entered historical costs. The ledger is
read and written through the Go API, never copied into localStorage. The Next.js
proxy explicitly forwards supported `/api` endpoints to `GO_API_URL` without caching
private responses. The Go backend continues to own scoring, collection jobs and
all database access. This UI does not turn source busy-time labels into demand.

The new ledger form explicitly interprets entered times in Australia/Melbourne,
regardless of the browser's time zone. It rejects the missing spring-forward hour;
a repeated autumn hour uses the earlier matching instant. Existing API timestamps
still require RFC3339 offsets. The native datetime inputs are labelled Melbourne
time. Browser preferences are separate from the legacy page's in-memory controls.

```sh
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend dev       # frontend only; Go must also be running
pnpm --dir frontend typecheck
pnpm --dir frontend test          # offline DST conversion and API proxy tests
pnpm --dir frontend build     # production build, including type checking
make build                         # builds both applications
make check                         # Go formatting/tests/build + frontend tests/types
```

A production frontend can use `pnpm --dir frontend start` after building,
or the standalone image in Compose. The browser still needs internet access for
OSM tiles; Leaflet and icons are bundled rather than loaded from third-party CDNs.
No API keys, hosted database, paid service or cloud deployment are required.

The frontend uses pnpm, pinned through `packageManager` in `frontend/package.json`.
`frontend/pnpm-lock.yaml` is the dependency lockfile; installs and Docker builds use
`--frozen-lockfile`. The Node 24 Docker builder enables Corepack to select the pinned
pnpm version. Native development uses the installed `pnpm` command.

### Full weekly opening hours

The Maps scraper now opens the primary `button[data-item-id="oh"]` (“See more
hours”) panel and waits up to ten seconds for **seven explicit weekday rows**,
instead of finishing as soon as today's row appears. It requests the standard
desktop layout using the installed Chromium version; Google's reduced headless
layout can otherwise contain only today's schedule. Browser profiles remain
isolated and logged out, and consent/challenge pages still stop collection.

The primary business-hours table supplies the schedule, including explicit closed
days, overnight ranges and multiple periods on the same day. All rows in that
table are read even if clipped or collapsed; kitchen/drive-through tables are not
merged into the primary schedule. Missing weekdays remain unavailable: a single
24-hour day never implies a 24/7 week. An incomplete response is saved as partial
and does not satisfy the fresh-week cache check.

Collection results now include `hours_days` (the number of weekdays observed in
that attempt) and `hours_complete` (all seven observed). `hours_collected` continues
to mean at least one day was found. Logs show the same coverage. Source rows in the
Next.js venue page display Monday–Sunday, a collected-day count and per-day dates;
missing days say “Not collected”. A missing busy chart is reported independently
from a complete opening-hours week.

SQLite merges fresh hours **by weekday in a transaction**, retaining previously
saved days when the new page omits them. Each `opening_hours` item now has a
`fetched_at` timestamp. Retained days keep their earlier timestamps; the top-level
`hours_fetched_at` is the latest successful hours collection, not proof that every
day was refreshed. Existing rows are upgraded lazily using their original component
timestamp. No database file replacement or delivery-history migration is needed.

Tests cover the supplied weekday-table structure, closed and split periods,
collapsed rows, separate kitchen hours, delayed weekly rendering, partial SQLite
refreshes and timestamp preservation. Browser fixtures make no public requests or
listening sockets. Popular-times extraction also recognizes plural day labels
such as “Tuesdays” and excludes graphs explicitly marked `aria-hidden="true"` so
other days' hidden chart labels are not assigned to the selected day.

### Date and time above the homepage map

The Explore page has a calendar date picker, 24-hour time selector and range
control immediately above the main map. The range accepts 0–100 km (0 includes all
cached coverage); blur the field or press Enter to apply it to the map circle,
venue list and rankings. It shares the range on the Settings page. Google Maps
collection continues to use its separate fixed 5 km radius. Changes automatically recalculate venue activity and rankings
using Australia/Melbourne time and persist with the shared scoring settings.
**Current hour** selects today's Melbourne date and hour at minute zero. Selecting
a weekday on the Settings page clears an explicit calendar date and restores the
existing next-occurrence weekday behavior. Without an explicit date, the map's
calendar displays that next occurrence using calendar arithmetic across DST.
The weather card continues to show the current cached weather observation.

### Collected opening hours in scoring

The main `/api/zones` and `/api/recommendations` responses now join collected
Google Maps schedules to OSM venues by stable OSM key. Schedule evaluation lives
in `internal/activity`, shared by API venue estimates, cluster optimization and
the grid scorer. The API enriches a request-local copy; original OSM schedules,
venue cache and delivery history are not overwritten. No browser collection or
public API call is triggered by this database join.

Each observed weekday is eligible for scoring for **24 hours after collection**,
measured against the real request time, independently of the selected planning
date. Fresh, strictly parsed Google opening hours take precedence over unknown
hours or an OSM open schedule. A confirmed OSM closure always remains zero;
conflicting Google open hours do not reopen it. Stale, missing, future-dated or
unsupported Google observations fall back to the OSM schedule and then the
existing low-confidence category/cuisine heuristic.

Supported Google text includes `Closed`, `Open 24 hours`, explicit AM/PM or 24-hour
clock intervals, comma-separated split periods, and overnight intervals. Opening
boundaries are inclusive and closing boundaries exclusive. Previous-day intervals
are checked using Melbourne calendar arithmetic across DST. A closed result needs
readable fresh data for today and the previous day, since an overnight interval
may spill into today; otherwise the status remains unknown and follows fallback.
Holiday qualifiers, relative labels and ambiguous clocks are not guessed. Weekly
schedules applied to future or historical dates remain planning assumptions and
do not prove the business's actual hours on those dates.

For example, the collected **Pelligra Cakes** schedule (7 am–4 pm Monday–Saturday,
Sunday closed) contributes zero before 7 am, from 4 pm onward and on Sunday. It
retains the usual cafe/meal/day factors while open. **No busy chart is required**:
a collection marked partial solely because popular times are unavailable can
still provide valid opening hours for scoring. Unknown busy data never implies
zero activity and busy-time labels are not converted into demand weights.

Per-venue `activity` now includes `hours_source` (`google_maps`, `osm`, `unknown`),
optional `hours_text` and `hours_url`, plus the existing reason/status/confidence.
Map popups display the source and schedule used. Supported schedules have medium
source confidence, not statistical certainty about popularity, orders or earnings.
Changes to collected schedules apply on the next scoring request; restart the
server after upgrading to load this implementation. Existing collected hours are
usable without recollecting them while their per-day timestamps remain fresh.
