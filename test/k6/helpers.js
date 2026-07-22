import { check } from 'k6';
import http from 'k6/http';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Base configuration
export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8000';

// Helper to generate UUID
export function generateUUID() {
  return uuidv4();
}

// Helper to generate future date range
export function generateFutureDateRange(hoursFromNow = 24, durationHours = 2) {
  const start = new Date();
  start.setHours(start.getHours() + hoursFromNow);

  const end = new Date(start);
  end.setHours(end.getHours() + durationHours);

  return {
    start: start.toISOString(),
    end: end.toISOString()
  };
}

// Helper to generate date for calendar queries
export function generateDateRange(daysBack = 7, daysForward = 30) {
  const from = new Date();
  from.setDate(from.getDate() - daysBack);

  const to = new Date();
  to.setDate(to.getDate() + daysForward);

  return {
    from: from.toISOString().split('T')[0],
    to: to.toISOString().split('T')[0]
  };
}

// Standard request headers
// AUTH_TOKEN: the API requires a Bearer token since auth landed; mint one via
// the OAuth stub (dev) and pass it as `k6 run -e AUTH_TOKEN=...`.
//
// Signup is invite-only by default, so a plain stub exchange yields a refusal,
// a guest (open signup), or — only on an empty database — the bootstrap admin.
// Mint the token WITH the dev-only X-Test-Roles header instead: it creates
// the stub user and grants the roles the scenarios need (admin covers the
// resource/maintenance/channel writes). From the host go through the proxy:
//
//   curl -s -X POST "http://localhost:9000/auth/api/v1/login/oauth/exchange/google" \
//     -H 'Content-Type: application/json' -H 'X-Test-Roles: admin' \
//     -d '{"id_token":"k6-load-test"}' | jq -r '.access_token'
export function getHeaders() {
  const headers = {
    'Content-Type': 'application/json',
  };
  if (__ENV.AUTH_TOKEN) {
    headers['Authorization'] = `Bearer ${__ENV.AUTH_TOKEN}`;
  }
  return headers;
}

// Check response helper
export function checkResponse(response, expectedStatus, testName) {
  return check(response, {
    [`${testName}: status is ${expectedStatus}`]: (r) => r.status === expectedStatus,
    [`${testName}: response time < 500ms`]: (r) => r.timings.duration < 500,
  });
}

// Resource types
export const RESOURCE_TYPES = ['service', 'database', 'cluster'];

// Maintenance scopes
export const MAINTENANCE_SCOPES = ['global', 'resource'];

// Maintenance impacts
export const MAINTENANCE_IMPACTS = ['none', 'partial_outage', 'full_outage'];

// Cancel reasons
export const CANCEL_REASONS = ['conflict', 'incident', 'business_decision', 'rescheduled', 'mistake'];

// Maintenance statuses
export const MAINTENANCE_STATUSES = ['draft', 'planned', 'in_progress', 'canceled', 'completed'];

// Random item selector
export function randomItem(array) {
  return array[Math.floor(Math.random() * array.length)];
}

// Generate resource payload
export function generateResourcePayload() {
  const type = randomItem(RESOURCE_TYPES);
  return {
    name: `test-${type}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    description: `Load test ${type} resource`,
    external_id: `ext-${generateUUID()}`
  };
}

// Create a fresh resource and return its id, or null on failure. Each
// maintenance scopes to its own brand-new resource so it never conflicts with
// any other maintenance — that keeps the approve conflicts_snapshot empty and
// stable (no drift-under-load 409s to retry around).
export function createResource(baseUrl, headers) {
  const res = http.post(
    `${baseUrl}/api/v1/resource/create`,
    JSON.stringify(generateResourcePayload()),
    { headers }
  );
  if (res.status !== 200) {
    return null;
  }
  const body = parseResponse(res);
  return body && body.id ? body.id : null;
}

// Generate maintenance payload. Pass the id of a freshly created resource so
// the window is scoped to it and stays conflict-free.
export function generateMaintenancePayload(resourceIds = []) {
  // Spread planned_start across a wide future range so even same-resource
  // windows don't overlap by accident.
  const plannedStart = new Date();
  plannedStart.setHours(
    plannedStart.getHours() + 24 + Math.floor(Math.random() * 24 * 90)
  );

  const scope = resourceIds.length > 0 ? 'resource' : 'global';

  const payload = {
    title: `Load Test Maintenance ${Date.now()}`,
    description: 'Automated load test maintenance window',
    scope: scope,
    impact: randomItem(MAINTENANCE_IMPACTS),
    planned_start: plannedStart.toISOString(),
    // approver_user_id and notify_targets are required by the current API;
    // the runner mints them against the target stand and passes them via
    // `k6 run -e APPROVER_ID=... -e CHANNEL_ID=...`.
    approver_user_id: __ENV.APPROVER_ID,
    notify_targets: { channel_ids: [__ENV.CHANNEL_ID] },
    steps: [{
      order: 1,
      description: 'Run load test maintenance task',
      rollback_description: 'Rollback load test maintenance task',
      duration: '2h'
    }]
  };

  if (resourceIds.length > 0) {
    payload.resources = resourceIds.map(id => ({ id: id }));
  }

  return payload;
}

// Parse response body safely
export function parseResponse(response) {
  try {
    return JSON.parse(response.body);
  } catch (e) {
    console.error(`Failed to parse response: ${response.body}`);
    return null;
  }
}

// Sleep helper with random jitter
export function sleepWithJitter(base, jitter = 1) {
  const sleep = base + Math.random() * jitter;
  return sleep;
}
