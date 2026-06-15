# MaintMode DB migrations image.
#
# Bakes the goose migration files into the kukymbr/goose-docker runner so the
# deploy repo can apply migrations on the VM without checking out backend
# source (it has no ./migrations dir of its own). This is the standalone
# analogue of compose.yaml's apply-migrations service, which mounts
# ./migrations at runtime — here the SQL ships INSIDE the image instead.
#
# The base image's entrypoint runs `goose -dir /migrations <DRIVER> <DSN> up`
# using GOOSE_DRIVER / GOOSE_DBSTRING from the environment. We only need to
# place the *.sql files where it looks.
#
# Build context is the repository root:
#   docker build -f deployment/migrations/Dockerfile -t maintmode-migrations .
#
# Pinned to match compose.yaml's apply-migrations service.
FROM kukymbr/goose-docker:3.26.0

# The base image reads migrations from /migrations by default.
COPY migrations/ /migrations/
