# MaintMode

MaintMode is a dashboard application with a calendar interface for managing and visualizing technical maintenance work schedules.

## Overview

MaintMode provides a centralized view of planned maintenance activities through an intuitive calendar interface, helping teams coordinate and track technical work schedules.

## Tech Stack

### Backend
- **Language**: Go 1.25.0+
- **Database**: PostgreSQL
- **HTTP Framework**: Echo
- **Database Libraries**:
  - `jet` - Type-safe SQL builder
  - `sqlx` - Extensions for database/sql
  - `goose` - Database migrations
- **Logging**: `zap` with `xlog` wrapper

### Frontend
- **Language**: TypeScript
- **Framework**: Svelte
- **Served**: Static files served by backend API

### Deployment
- Delivered as Docker image
- Backend serves static UI files through API endpoints

## Development

### Prerequisites
- Go 1.25.0 or later
- PostgreSQL
- Docker (for deployment)

### Building
```bash
make build
```

### Testing
```bash
make test
```

### Linting
```bash
make lint
```

## Documentation

For developers and AI agents working on this project, see the `.agent/` directory for detailed conventions, workflows, and coding standards.
