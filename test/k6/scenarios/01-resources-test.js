import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  getHeaders,
  checkResponse,
  generateResourcePayload,
  parseResponse,
  sleepWithJitter
} from '../helpers.js';

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 5 },   // Ramp up to 5 users
    { duration: '1m', target: 10 },   // Stay at 10 users
    { duration: '30s', target: 20 },  // Peak load
    { duration: '1m', target: 10 },   // Scale back
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.05'], // Less than 5% errors
  },
};

export default function() {
  const headers = getHeaders();
  let createdResourceId = null;

  // 1. Create Resource
  const createPayload = generateResourcePayload();
  const createRes = http.post(
    `${BASE_URL}/api/v1/resource/create`,
    JSON.stringify(createPayload),
    { headers }
  );

  const createSuccess = checkResponse(createRes, 200, 'Create Resource');
  if (createSuccess && createRes.status === 200) {
    const body = parseResponse(createRes);
    if (body && body.id) {
      createdResourceId = body.id;
    }
  }

  sleep(sleepWithJitter(1, 0.5));

  // 2. Search Resources
  const searchRes = http.get(
    `${BASE_URL}/api/v1/resources?name=${encodeURIComponent(createPayload.name.substring(0, 10))}`,
    { headers }
  );

  checkResponse(searchRes, 200, 'Search Resources');
  check(searchRes, {
    'Search: has resources array': (r) => {
      const body = parseResponse(r);
      return body && Array.isArray(body.resources);
    }
  });

  sleep(sleepWithJitter(0.5, 0.3));

  // 3. Get Resource Types (if resource was created)
  if (createdResourceId) {
    const typesRes = http.get(
      `${BASE_URL}/api/v1/resource/${createdResourceId}/types`,
      { headers }
    );

    checkResponse(typesRes, 200, 'Get Resource Types');
    check(typesRes, {
      'Types: has types array': (r) => {
        const body = parseResponse(r);
        return body && Array.isArray(body.types);
      }
    });
  }

  sleep(sleepWithJitter(1, 0.5));
}
