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
  check(createRes, {
    'Create: has maintenance ID': (r) => {
      const body = parseResponse(r);
      return body && body.id;
    },
    'Create: status is draft': (r) => {
      const body = parseResponse(r);
      return body && body.status === 'draft';
    }
  });

  if (createSuccess && createRes.status === 200) {
    const body = parseResponse(createRes);
    if (body && body.id) {
      maintenanceId = body.id;
    }
  }

  sleep(sleepWithJitter(1, 0.5));

  // 2. Get Maintenance by ID
  if (maintenanceId) {
    const getRes = http.get(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
      { headers }
    );

    checkResponse(getRes, 200, 'Get Maintenance');
    check(getRes, {
      'Get: returns correct ID': (r) => {
        const body = parseResponse(r);
        return body && body.id === maintenanceId;
      }
    });

    sleep(sleepWithJitter(0.5, 0.3));

    // 3. Update Maintenance Draft
    const updatePayload = generateMaintenancePayload();
    updatePayload.title = `Updated ${createPayload.title}`;

    const updateRes = http.post(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}/edit`,
      JSON.stringify(updatePayload),
      { headers }
    );

    checkResponse(updateRes, 204, 'Update Maintenance');

    sleep(sleepWithJitter(0.5, 0.3));

    // 4. Verify update
    const verifyRes = http.get(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
      { headers }
    );

    checkResponse(verifyRes, 200, 'Verify Update');
    check(verifyRes, {
      'Verify: title was updated': (r) => {
        const body = parseResponse(r);
        return body && body.title === updatePayload.title;
      }
    });
  }

  sleep(sleepWithJitter(1, 0.5));
}
