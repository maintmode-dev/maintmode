# Makefile Integration

Docker commands integrated into Makefile for streamlined development workflow.

## Database Services

Commands for managing database containers:

```makefile
# docker-up - Start all database services
.PHONY: docker-up
docker-up:
	docker-compose up -d
	make docker-ps

# docker-down - Stop and remove all database containers
.PHONY: docker-down
docker-down:
	docker-compose down -v --remove-orphans
	make docker-ps

# docker-logs - Stream logs from all database containers
.PHONY: docker-logs
docker-logs:
	docker-compose logs -f

# docker-ps - Show status of database containers
.PHONY: docker-ps
docker-ps:
	docker-compose ps -a
```

## Application with All Services

Commands for managing the full application stack:

```makefile
# app-up - Start all services with the application
.PHONY: app-up
app-up:
	docker-compose -f compose.yaml -f compose.app.yaml up -d
	make app-ps

# app-down - Stop and remove all containers
.PHONY: app-down
app-down:
	docker-compose -f compose.yaml -f compose.app.yaml down -v --remove-orphans
	make app-ps

# app-logs - Stream logs from application container
.PHONY: app-logs
app-logs:
	docker-compose -f compose.yaml -f compose.app.yaml logs -f app

# app-ps - Show status of all containers
.PHONY: app-ps
app-ps:
	docker-compose -f compose.yaml -f compose.app.yaml ps -a
```

## Usage Examples

### Start database services:
```bash
make docker-up
```

### View database logs:
```bash
make docker-logs
```

### Start full application stack:
```bash
make app-up
```

### View application logs:
```bash
make app-logs
```

### Stop everything:
```bash
make app-down
```

## Best Practices

1. Always check container status after up/down operations (`make docker-ps`)
2. Use `-d` flag for detached mode in production
3. Include `--remove-orphans` to clean up old containers
4. Provide clear, descriptive help comments for each target
5. Group related commands (database vs. full app)
6. Use `.PHONY` to ensure targets always run
