import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  getHeaders,
  checkResponse,
  generateDateRange,
  parseResponse,
  sleepWithJitter,
  randomItem,
  MAINTENANCE_STATUSES
} from '../helpers.js';

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 20 },
    { duration: '30s', target: 30 },
    { duration: '1m', target: 20 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};

export default function() {
  const headers = getHeaders();

  // 1. Get calendar view without filters
  const dateRange = generateDateRange(7, 30);
  const basicRes = http.get(
    `${BASE_URL}/ui/v1/calendar?from=${dateRange.from}&to=${dateRange.to}`,
    { headers }
  );

  checkResponse(basicRes, 200, 'Calendar Basic');
  check(basicRes, {
    'Calendar: has events array': (r) => {
      const body = parseResponse(r);
      return body && Array.isArray(body.events);
    },
    'Calendar: has meta': (r) => {
      const body = parseResponse(r);
      return body && body.meta;
    }
  });

  sleep(sleepWithJitter(0.5, 0.3));

  // 2. Get calendar with status filter
  const statuses = [randomItem(MAINTENANCE_STATUSES), randomItem(MAINTENANCE_STATUSES)];
  const statusFilterRes = http.get(
    `${BASE_URL}/ui/v1/calendar?from=${dateRange.from}&to=${dateRange.to}&statuses=${statuses.join('&statuses=')}`,
    { headers }
  );

  checkResponse(statusFilterRes, 200, 'Calendar with Status Filter');

  sleep(sleepWithJitter(0.5, 0.3));

  // 3. Get short date range (1 week)
  const shortRange = generateDateRange(0, 7);
  const shortRes = http.get(
    `${BASE_URL}/ui/v1/calendar?from=${shortRange.from}&to=${shortRange.to}`,
    { headers }
  );

  checkResponse(shortRes, 200, 'Calendar Short Range');

  sleep(sleepWithJitter(1, 0.5));
}
