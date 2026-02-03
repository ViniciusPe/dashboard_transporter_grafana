# Dashboard Transporter for Grafana

A dual-component system for transporting Grafana dashboards between environments.

## Components

- **Backend**: Go HTTP service that manages Grafana API interactions
- **Frontend**: React Grafana plugin that provides UI within Grafana
- **Development Environment**: Docker Compose setup with multiple Grafana instances

## Quick Start

### Prerequisites

- Node.js >= 22 (for plugin development)
- Go 1.21+ (for backend)
- Docker and Docker Compose (for running the environment)

### Setup

1. **Build the Plugin**

   ```bash
   cd plugin/brade-dashboardtransporter-app
   npm install
   npm run build
   ```

   This creates the `dist/` directory with the compiled plugin.

2. **Start the Environment**

   ```bash
   cd docker
   docker compose up
   ```

   This starts:
   - `grafana-dev` on http://localhost:3001
   - `grafana-hml` on http://localhost:3002
   - `grafana-prd` on http://localhost:3003
   - Backend API on http://localhost:8080

3. **Access Grafana**

   - Login: `admin` / `admin`
   - The Dashboard Transporter plugin will be available in the sidebar

## Development

### Plugin Development

```bash
cd plugin/brade-dashboardtransporter-app
npm run dev  # Watch mode
```

### Backend Development

```bash
cd backend
go run cmd/server/main.go
```

## Documentation

- [Architecture](docs/architecture.md)
- [API Documentation](docs/api.md)
- [Governance](docs/governance.md)
