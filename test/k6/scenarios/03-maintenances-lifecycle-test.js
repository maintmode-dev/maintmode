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
    { duration: '30s', target: 3 },
    { duration: '1m', target: 5 },
    { duration: '30s', target: 8 },
    { duration: '1m', target: 5 },
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
  let revision = 0;

  // 1. Create Maintenance Draft
  const createPayload = generateMaintenancePayload();
  const createRes = http.post(
    `${BASE_URL}/api/v1/maintenances/create`,
    JSON.stringify(createPayload),
    { headers }
  );

  const createSuccess = checkResponse(createRes, 200, 'Create Draft');
  if (createSuccess && createRes.status === 200) {
    const body = parseResponse(createRes);
    if (body && body.id) {
      maintenanceId = body.id;
    }
  }

  if (!maintenanceId) {
    return; // Cannot proceed without ID
  }

  sleep(sleepWithJitter(1, 0.5));

  // 2. Get maintenance to fetch revision
  const getRes = http.get(
    `${BASE_URL}/ui/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  const viewSuccess = checkResponse(getRes, 200, 'Get Maintenance View');
  let conflicts = [];
  if (viewSuccess && getRes.status === 200) {
    const body = parseResponse(getRes);
    if (body && body.maintenance) {
      revision = body.maintenance.revision || 0;
    }
    if (body && body.conflicts) {
      conflicts = body.conflicts;
    }
  }

  sleep(sleepWithJitter(0.5, 0.3));

  // 3. Approve Maintenance
  const approvePayload = {
    observed_maint_revision: revision,
    conflicts_snapshot: conflicts
  };

  const approveRes = http.post(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}/approve`,
    JSON.stringify(approvePayload),
    { headers }
  );

  const approveSuccess = checkResponse(approveRes, 204, 'Approve Maintenance');

  // Only proceed if approve was successful
  if (!approveSuccess || approveRes.status !== 204) {
    return; // Cannot proceed without approval
  }

  sleep(sleepWithJitter(1, 0.5));

  // 4. Start Maintenance
  const startRes = http.post(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}/start`,
    null,
    { headers }
  );

  const startSuccess = checkResponse(startRes, 204, 'Start Maintenance');

  // Only complete if start was successful
  if (!startSuccess || startRes.status !== 204) {
    return; // Cannot complete without starting
  }

  sleep(sleepWithJitter(2, 1));

  // 5. Complete Maintenance
  const completeRes = http.post(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}/complete`,
    null,
    { headers }
  );

  checkResponse(completeRes, 204, 'Complete Maintenance');

  sleep(sleepWithJitter(0.5, 0.3));

  // 6. Verify final state
  const finalRes = http.get(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  checkResponse(finalRes, 200, 'Verify Final State');
  check(finalRes, {
    'Final: status is completed': (r) => {
      const body = parseResponse(r);
      return body && body.status === 'completed';
    }
  });

  sleep(sleepWithJitter(1, 0.5));
}
