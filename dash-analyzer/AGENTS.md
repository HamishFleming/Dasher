# Working on Dash Analyzer

This is a Go service with a plain Leaflet dashboard. Work directly in this repository.
Use free public APIs only. Never invent DoorDash demand, participation, driver counts,
orders or earnings. Explain all heuristics and label estimated travel separately from
measured trips. Preserve cached data on upstream failure and do not call public APIs
from tests; use httptest fixtures and temporary SQLite databases.

Keep Melbourne time zone handling DST aware. Monetary values are AUD; distance is km.
Driver-entered history is private and should stay local. Do not commit data/ or DBs.

Before completion run gofmt -w cmd internal, go test ./..., and go build ./....
Update README when changing APIs, assumptions or operating instructions.

Activity scoring lives in internal/activity and must be shared by the grid analyzer,
cluster optimizer and API venue estimates. Interpret selected times in
Australia/Melbourne using calendar arithmetic; never add fixed 24-hour durations
to select weekdays across DST. Day query values are 0=Sunday through 6=Saturday.
Confirmed closed OSM schedules contribute zero; missing or unsupported schedules
remain unknown with low confidence. Do not equate schedule coverage with statistical
certainty or invent Google Maps popularity. Keep meal profiles and cuisine/day
multipliers documented in README. Preserve venue markers and parking controls.
Use in-process httptest fixtures so tests need neither public APIs nor listening
sockets. The third_party/openhours snapshot retains its upstream license; preserve
provenance and the strict subset guard when updating this permissive parser.
