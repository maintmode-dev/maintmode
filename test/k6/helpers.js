import { check } from 'k6';
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
export function getHeaders() {
  return {
    'Content-Type': 'application/json',
  };
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

// Generate maintenance payload
export function generateMaintenancePayload(resourceIds = []) {
  const plannedStart = new Date();
  plannedStart.setHours(plannedStart.getHours() + 24);

  const scope = resourceIds.length > 0 ? 'resource' : 'global';

  const payload = {
    title: `Load Test Maintenance ${Date.now()}`,
    description: 'Automated load test maintenance window',
    scope: scope,
    impact: randomItem(MAINTENANCE_IMPACTS),
    planned_start: plannedStart.toISOString(),
    steps: [{
      order: 1,
      description: 'Run load test maintenance task',
      rollback_description: 'Rollback load test maintenance task',
      duration: '2h'
    }]
  };

  if (resourceIds.length > 0) {
    payload.resources = resourceIds.map(id => ({
      id: id,
      type: randomItem(RESOURCE_TYPES)
    }));
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
