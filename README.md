# halo

<p align="center">
  <img src="https://page.luistavares.pt/files/elephant.gif" alt="halo logo" height="128" />
</p>

<p align="center">
  <strong>A lightweight, provider-agnostic SQL-over-HTTP proxy for PostgreSQL</strong>
</p>

<p align="center">
  <a href="https://github.com/luist18/halo/actions"><img src="https://github.com/luist18/halo/workflows/Go/badge.svg" alt="CI Status"></a>
  <a href="https://github.com/luist18/halo/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go" alt="Go Version">
</p>

---

`halo` (or `halo-proxy`) is a lightweight proxy for PostgreSQL, designed to work seamlessly with the [Neon serverless driver](https://github.com/neondatabase/serverless). It enables you to use the Neon serverless driver with **any** PostgreSQL database—whether it's a self-managed instance on a VPS, a cluster on Kubernetes, or even a local development container.

## Why halo?

Some popular serverless environments (like Vercel Edge Functions) restrict the use of raw TCP sockets, which traditional PostgreSQL drivers require. The Neon serverless driver solves this by using HTTP or WebSockets as the transport layer instead.

**halo** brings this capability to any PostgreSQL setup by acting as a translation layer:

```
┌─────────────────┐        HTTP/JSON        ┌──────────┐      PostgreSQL      ┌────────────┐
│  Your App       │ ───────────────────────▶│  halo    │ ───────────────────▶ │  PostgreSQL│
│  (Edge Runtime) │                         │  proxy   │   Wire Protocol      │  Database  │
└─────────────────┘                         └──────────┘                      └────────────┘
```

Read the full motivation in the [WHY.md](WHY.md) document.

## Features

- 🚀 **SQL-over-HTTP** — Execute PostgreSQL queries via simple HTTP POST requests
- 🔗 **Connection Pooling** — Built-in connection pool with TTL-based cache management
- 📦 **Batch Queries** — Execute multiple queries in a single transaction
- 🔒 **Transaction Control** — Configurable isolation levels, read-only, and deferrable modes
- 🎯 **Neon Driver Compatible** — Drop-in support for the Neon serverless driver
- ⚡ **Lightweight** — Single binary with minimal dependencies
- 🛠️ **Embeddable** — Use as a standalone proxy or embed into your Go application

## Quick Start

### Running the Proxy

```bash
# Clone the repository
git clone https://github.com/luist18/halo.git
cd halo/halo

# Run the proxy
go run main.go
```

The proxy will start on port `8080` with the SQL endpoint at `/sql`.

### Basic Usage

Send a POST request with your SQL query:

```bash
curl -X POST http://localhost:8080/sql \
  -H "Content-Type: application/json" \
  -H "Neon-Connection-String: postgres://user:password@localhost:5432/mydb" \
  -d '{"query": "SELECT * FROM users WHERE id = $1", "params": [1]}'
```

### Using with Neon Serverless Driver

```typescript
import { neon } from '@neondatabase/serverless';

// Point the driver to your halo proxy
const sql = neon('postgres://user:password@localhost:5432/mydb', {
  fetchEndpoint: 'http://localhost:8080/sql',
});

// Use it just like you would with Neon
const users = await sql`SELECT * FROM users`;
```

## API Reference

### Endpoint

```
POST /sql
```

### Headers

| Header | Description | Default |
|--------|-------------|---------|
| `Neon-Connection-String` | PostgreSQL connection string (required) | — |
| `Neon-Pool-Opt-In` | Enable connection pooling | `true` |
| `Neon-Batch-Isolation-Level` | Transaction isolation level for batch queries | `ReadCommitted` |
| `Neon-Batch-Read-Only` | Set transaction to read-only | `false` |
| `Neon-Batch-Deferrable` | Set transaction to deferrable | `false` |

### Request Body

**Single Query:**
```json
{
  "query": "SELECT * FROM users WHERE id = $1",
  "params": [1]
}
```

**Batch Queries:**
```json
{
  "queries": [
    { "query": "INSERT INTO users (name) VALUES ($1)", "params": ["Alice"] },
    { "query": "INSERT INTO users (name) VALUES ($1)", "params": ["Bob"] }
  ]
}
```

### Response

**Single Query Response:**
```json
{
  "fields": [
    { "name": "id", "dataTypeID": 23 },
    { "name": "name", "dataTypeID": 25 }
  ],
  "rows": [
    [1, "Alice"]
  ],
  "command": "SELECT",
  "rowCount": 1,
  "rowAsArray": true
}
```

**Batch Query Response:**
```json
{
  "results": [
    { "fields": [...], "rows": [...], "command": "INSERT", "rowCount": 1, "rowAsArray": true },
    { "fields": [...], "rows": [...], "command": "INSERT", "rowCount": 1, "rowAsArray": true }
  ]
}
```

### Connection String Formats

halo supports both URI and keyword-value formats:

```
# URI format
postgresql://user:password@host:5432/database?sslmode=require

# Keyword-value format
host=localhost port=5432 dbname=mydb user=myuser password=secret
```

### Limits

| Limit | Value |
|-------|-------|
| Max payload size | 10 MB |
| Max batch queries | 1,024 |
| Max query length | 100 KB |
| Connection pool TTL | 10 minutes |

## Project Structure

```
halo/
├── main.go                     # Application entry point
├── httpexecutor/               # Query execution engine
│   ├── executor.go             # Core query execution logic
│   ├── connection_pool.go      # Connection pool management
│   └── parser_util.go          # SQL parsing utilities
├── proxy/http/                 # HTTP proxy layer
│   ├── proxy.go                # HTTP server and routing
│   ├── headers.go              # Header parsing
│   ├── payload.go              # Request payload handling
│   ├── middleware.go           # HTTP middleware
│   └── internal/errors/        # Error handling
└── internal/                   # Internal packages
    ├── cache/                  # TTL cache implementation
    ├── connstr/                # Connection string parser
    └── data/                   # Data structures (OrderedMap, Secret)
```

## Development

### Prerequisites

- Go 1.25+
- PostgreSQL (for integration tests)

### Running Tests

```bash
cd halo

# Run all tests
go test ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting

```bash
golangci-lint run
```

## Roadmap

- [ ] Connection string whitelisting (regex or hostname-based)
- [ ] Runtime configuration via ctl utility
- [ ] Helm chart for Kubernetes deployment
- [ ] WebSocket support
- [ ] Configurable proxy parameters (max-payload, endpoints, transport modes)

## License

`halo` is open source software, licensed under the [MIT License](LICENSE).
