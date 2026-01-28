# AI Coding Instructions: Dashboard Transporter

## Architecture Overview

This is a **dual-component Grafana dashboard transport system**:
- **Backend**: Go HTTP service (`backend/`) using chi router that manages Grafana API interactions
- **Frontend**: React Grafana plugin (`plugin/brade-dashboardtransporter-app/`) that provides UI within Grafana
- **Development Environment**: Docker Compose setup with two Grafana instances (dev:3001, hml:3002)

## Key Service Boundaries

### Backend Structure (`backend/`)
- **Entry point**: `cmd/server/main.go` - starts HTTP server on `:8080` with CORS middleware
- **HTTP layer**: `internal/http/` - chi router with handlers in `handlers/` subdirectory
- **Grafana integration**: `internal/grafana/` - HTTP client for Grafana APIs using Basic Auth
- **Configuration**: `internal/config/config.go` - environment-based config loading from env vars
- **Transport logic**: `internal/transport/` - dashboard transformation/sanitization logic

### Plugin Structure (`plugin/`)
- **Grafana App Plugin**: React TypeScript app that runs inside Grafana
- **Build system**: Webpack-based with TypeScript, uses `npm run dev` for development
- **Pages**: Multiple route-based pages (`PageOne`, `PageTwo`, etc.) with lazy loading
- **Plugin manifest**: `src/plugin.json` defines app metadata and navigation

## Critical Development Patterns

### Environment Configuration
```go
// Environments loaded from env vars - NO hardcoded credentials
URL:      os.Getenv("GRAFANA_DEV_URL"),
User:     os.Getenv("GRAFANA_DEV_USER"),  // Currently hardcoded as "admin"
Password: os.Getenv("GRAFANA_DEV_PASSWORD"), // Currently hardcoded as "admin"
```

### API Patterns
- **Backend APIs**: Follow `/api/{resource}` pattern with query params (e.g., `?env=dev`)
- **Grafana Client**: Custom HTTP client in `grafana/client.go` using Basic Auth header
- **Error handling**: Standard HTTP status codes with descriptive messages

### Frontend-Backend Communication
```tsx
// Frontend calls backend directly
fetch('http://localhost:8080/dashboards?env=dev')
```

## Development Workflows

### Backend Development
```bash
# From backend/ directory
go run cmd/server/main.go  # Starts server on :8080
```

### Plugin Development
```bash
# From plugin/brade-dashboardtransporter-app/
npm run dev    # Webpack watch mode
npm run build  # Production build
npm run server # Starts docker compose environment
```

### Docker Environment
```bash
# From project root
docker compose -f docker/docker-compose.yml up
# Creates: grafana-dev (3001), grafana-hml (3002)
```

## Integration Points

### Grafana API Integration
- Uses `/api/search` for dashboard listing
- Uses `/api/dashboards/uid/{uid}` for dashboard details
- **Auth**: Basic Auth with admin/admin (development setup)
- **Client pattern**: `grafana.NewClient(baseURL, username, password)`

### Plugin Registration
- Plugin ID: `brade-dashboardtransporter-app`
- Requires `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=dashboard_transporter`
- Navigation: Auto-added to Grafana sidebar with Admin role requirement

## Project-Specific Conventions

### File Organization
- **Backend handlers**: One file per resource (`dashboards.go`, `environments.go`)
- **Plugin pages**: Route-based with lazy loading imports
- **Docker volumes**: Separate dev/hml plugin directories in `docker/volumes/`

### Import/Export Pattern
- **List dashboards** → **Select multiple** → **Batch import** via POST `/dashboards/import/batch`
- **Transport service**: Located in `internal/transport/` (currently minimal)
- **Batch operations**: Expected pattern for dashboard operations

### Development Notes
- Backend uses **chi router** (not Gin/Echo)
- Plugin built with **@grafana/plugin-e2e** for testing
- **CORS enabled** for development (marked for removal in production)
- Environment switching via `env` query parameter in API calls