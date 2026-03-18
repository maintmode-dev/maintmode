import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  getHeaders,
  checkResponse,
  generateMaintenancePayload,
  parseResponse,
  sleepWithJitter,
  randomItem,
  CANCEL_REASONS
} from '../helpers.js';

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 3 },
    { duration: '1m', target: 7 },
    { duration: '30s', target: 10 },
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
    return;
  }

  sleep(sleepWithJitter(1, 0.5));

  // 2. Get maintenance view to fetch revision and conflicts
  const viewRes = http.get(
    `${BASE_URL}/ui/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  let revision = 0;
  let conflicts = [];
  if (viewRes.status === 200) {
    const body = parseResponse(viewRes);
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

  checkResponse(approveRes, 204, 'Approve Maintenance');

  sleep(sleepWithJitter(1, 0.5));

  // 4. Cancel Maintenance
  const cancelPayload = {
    reason: randomItem(CANCEL_REASONS),
    comment: 'Load test cancellation'
  };

  const cancelRes = http.post(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}/cancel`,
    JSON.stringify(cancelPayload),
    { headers }
  );

  checkResponse(cancelRes, 204, 'Cancel Maintenance');

  sleep(sleepWithJitter(0.5, 0.3));

  // 5. Verify cancellation
  const verifyRes = http.get(
    `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
    { headers }
  );

  checkResponse(verifyRes, 200, 'Verify Cancellation');
  check(verifyRes, {
    'Verify: status is canceled': (r) => {
      const body = parseResponse(r);
      return body && body.status === 'canceled';
    },
    'Verify: has cancel reason': (r) => {
      const body = parseResponse(r);
      return body && body.cancel_reason === cancelPayload.reason;
    }
  });

  sleep(sleepWithJitter(1, 0.5));
}
