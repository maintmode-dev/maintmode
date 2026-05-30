# Custom Caddy image with the caddy-ratelimit module baked in.
#
# The stock `caddy:2-alpine` image does NOT include
# github.com/mholt/caddy-ratelimit, so the `rate_limit` directive used in
# our Caddyfile / Caddyfile.prod is unavailable there. We build Caddy from
# the official builder image via xcaddy with the module added.
#
# Versions are pinned deliberately (not :latest / @master) for reproducible
# builds — bump CADDY_VERSION / RATELIMIT_VERSION intentionally.
#   - Caddy 2.11.3            (matches caddy:2.11.3-* on Docker Hub)
#   - caddy-ratelimit v0.1.0  (latest tagged release, 2024-08-28)
#
# Built by compose (see compose.yaml `caddy.build`) for local/dev/prod on the
# single VM. No separate registry push: the perimeter limiter lives entirely
# in this image + the mounted Caddyfile.

ARG CADDY_VERSION=2.11.3

# --- Stage 0: build caddy with the rate-limit module ---
FROM caddy:${CADDY_VERSION}-builder-alpine AS builder

ARG RATELIMIT_VERSION=v0.1.0
RUN xcaddy build \
    --with github.com/mholt/caddy-ratelimit@${RATELIMIT_VERSION}

# --- Stage 1: runtime image (same base as stock caddy:2-alpine) ---
FROM caddy:${CADDY_VERSION}-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
