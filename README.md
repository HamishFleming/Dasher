# Dash Analyzer

A local planning dashboard for Melbourne delivery drivers. Explore venue clusters, compare travel and fuel assumptions, collect venue opening hours, and track the earnings from your completed deliveries.

Built with **Next.js, React, TypeScript, Go and SQLite**, with Leaflet maps, OpenStreetMap venues and Open-Meteo weather. The dashboard requires no API keys or paid services.

> Scores are planning heuristics, not live DoorDash demand, confirmed platform participation, order predictions or promised income. This project has no DoorDash integration.

[Dashboard](images/Screenshot%202026-11-10%20at%2022-53-19%20Explore%20·%20Dash%20Analyzer.png)
![Dash Analyzer Explore dashboard](images/Screenshot%202026-11-10%20at%2022-51-39%20Explore%20%C2%B7%20Dash%20Analyzer.png)

## Quickstart / Demo

With Go 1.25+, Node.js 24+, pnpm 12+ and Make installed, run from the repository root:

```sh
cd dash-analyzer
make run
```

Open **http://127.0.0.1:3000**. The Go API runs at **http://127.0.0.1:8080**. Press Ctrl+C to stop both services.

Initial venue collection runs in the background. Internet access is needed for public data and map tiles; rankings can be empty until venue data arrives.

## Table of contents

- [Installation](#installation)
- [Usage](#usage)
- [Screenshots](#screenshots)
- [Development](#development)
- [Contribute](#contribute)
- [License](#license)

## Installation

### Native

Install the prerequisites above, then use `make run` inside `dash-analyzer/`. The command installs locked frontend dependencies when needed and starts both applications.

To start services in separate terminals, run `make api` and `make frontend` from that directory. The original interface is also available at **http://127.0.0.1:8080/web/**.

### Docker

With Docker Compose installed, run from the repository root:

```sh
cd dash-analyzer
docker compose up --build -d
```

Open **http://127.0.0.1:8080**. Compose publishes the frontend on localhost and keeps the Go API on its internal network.

```sh
docker compose down
```

The `analyzer-data` volume retains the SQLite ledger and venue cache between runs. Removing that volume deletes the stored history. Native runs store files in `dash-analyzer/data/`; stop the server before backing up that directory.

### Configuration

| Variable | Purpose | Default |
| --- | --- | --- |
| `FRONTEND_PORT` | Native development frontend port | `3000` |
| `PORT` | Go API port | `8080` |
| `LISTEN_ADDR` | Go listen address | `127.0.0.1:8080` |
| `GO_API_URL` | Server-side frontend proxy destination | `http://127.0.0.1:8080` |
| `DATA_DIR` | Native database and cache directory, relative to the application directory | `data` |

The application has no authentication. Keep it on localhost or add an authenticated reverse proxy before sharing access.

## Usage

| Page | What you can do |
| --- | --- |
| **Explore** (`/`) | Select your origin, planning date, hour and range; inspect venue markers and ranked clusters on the map. |
| **Venue intelligence** (`/collection`) | Collect optional Google Maps observations within 5 km, monitor or cancel a job, and inspect cached opening hours and popular-times observations. |
| **Delivery ledger** (`/history`) | Record completed deliveries or work blocks, then review gross earnings, costs, net income, net/hour and net/km. |
| **Scoring settings** (`/settings`) | Adjust origin, meal time, parking sensitivity, grocery weighting, fuel consumption, fuel price and road-distance assumptions. |

1. Set a Melbourne origin and your vehicle assumptions.
2. Choose a planning date and time, then inspect the map and cluster rankings. Times use `Australia/Melbourne`, including daylight saving.
3. Optionally collect venue observations to improve opening-hours coverage. Missing observations remain unknown.
4. Log completed work with elapsed time, distance, fuel and other costs. Include waiting and repositioning for meaningful hourly results.

Preferences stay in browser localStorage; delivery history stays in SQLite. Historical costs use the values you recorded rather than current fuel settings.

Coverage is a fixed Melbourne metropolitan bounding box. Travel estimates use distance assumptions rather than road routing or live traffic. Weather is a current cached observation even when planning another date. See the [detailed application documentation](dash-analyzer/README.md) for scoring, freshness rules, collector setup and REST API examples.

## Screenshots

### Venue intelligence

![Venue intelligence and collected observations](images/Screenshot%202026-11-10%20at%2022-50-37%20Venue%20intelligence%20%C2%B7%20Dash%20Analyzer.png)

### Delivery ledger

![Delivery ledger and earnings analytics](images/Screenshot%202026-11-10%20at%2022-50-45%20Delivery%20ledger%20%C2%B7%20Dash%20Analyzer.png)

### Scoring settings

![Scoring settings](images/Screenshot%202026-11-10%20at%2022-53-31%20Scoring%20settings%20%C2%B7%20Dash%20Analyzer.png)

More captures, including the original interface, are available in [images/](images/).

## Development

Run these commands inside `dash-analyzer/`:

```sh
make build   # Build the Go server and production frontend
make test    # Run Go and frontend tests
make check   # Format Go files, run tests, build Go and check frontend types
```

For focused frontend checks:

```sh
pnpm --dir frontend typecheck
pnpm --dir frontend test
pnpm --dir frontend build
```

| Directory | Contents |
| --- | --- |
| `dash-analyzer/cmd/` | Go command entry points |
| `dash-analyzer/internal/` | Backend scoring, data collection, persistence and HTTP services |
| `dash-analyzer/frontend/` | Next.js dashboard and same-origin API proxy |
| `images/` | Dashboard screenshots |

The [application README](dash-analyzer/README.md) covers API endpoints, optional browser collection, data sources and implementation details.

## Contribute

Open an issue describing the problem or proposed improvement. For code changes, keep the scope focused, include relevant validation, and run `make check` before submitting a pull request. Use synthetic fixtures rather than personal delivery records in examples and tests.

## License

This repository does not currently include a project-wide license file. Vendored dependencies retain their own licenses, including [openhours](dash-analyzer/third_party/openhours/LICENSE).

README structure adapted from [pottekkat/awesome-readme](https://github.com/pottekkat/awesome-readme).
