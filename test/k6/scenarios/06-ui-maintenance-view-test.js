import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  getHeaders,
  checkResponse,
  generateMaintenancePayload,
  parseResponse,
  sleepWithJitter
} from '../helpers.js';

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 5 },
    { duration: '1m', target: 10 },
    { duration: '30s', target: 15 },
    { duration: '1m', target: 10 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};

export default function() {
  const headers = getHeaders();
  let maintenanceId = null;

  // 1. Create Maintenance Draft
  const createPayload = generateMaintenancePayload();
  const createRes = http.post(
    `${BASE_URL}/api/v1/maintenances/create`,
    JSON.stringify(createPayload),
    { headers }
  );

  const createSuccess = checkResponse(createRes, 200, 'Create Maintenance');
  if (createSuccess && createRes.status === 200) {
    const body = parseResponse(createRes);
    if (body && body.id) {
      maintenanceId = body.id;
    }
  }

  if (!maintenanceId) {
    return;
  }

  sleep(sleepWithJitter(1, 0.5));

  // 2. Get UI Maintenance View
  const viewRes = http.get(
    `${BASE_URL}/ui/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  checkResponse(viewRes, 200, 'Get Maintenance View');
  check(viewRes, {
    'View: has maintenance object': (r) => {
      const body = parseResponse(r);
      return body && body.maintenance;
    },
    'View: has actions': (r) => {
      const body = parseResponse(r);
      return body && body.actions;
    },
    'View: has conflicts array': (r) => {
      const body = parseResponse(r);
      return body && Array.isArray(body.conflicts);
    },
    'View: can edit draft': (r) => {
      const body = parseResponse(r);
      return body && body.actions && body.actions.can_edit === true;
    },
    'View: can approve draft': (r) => {
      const body = parseResponse(r);
      return body && body.actions && body.actions.can_approve === true;
    }
  });

  sleep(sleepWithJitter(1, 0.5));

  // 3. Get view again (simulating UI polling)
  const refreshRes = http.get(
    `${BASE_URL}/ui/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  checkResponse(refreshRes, 200, 'Refresh Maintenance View');

  sleep(sleepWithJitter(0.5, 0.3));
}
