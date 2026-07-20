import http from 'k6/http';
import { check, sleep } from 'k6';
import { group } from 'k6';
import {
  BASE_URL,
  getHeaders,
  checkResponse,
  generateResourcePayload,
  generateMaintenancePayload,
  createResource,
  generateDateRange,
  parseResponse,
  sleepWithJitter,
  randomItem,
  CANCEL_REASONS
} from '../helpers.js';

// Test configuration - simulating multiple engineers working simultaneously
export const options = {
  scenarios: {
    // Engineer 1: Creating and managing resources
    resource_manager: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 3 },
        { duration: '2m', target: 5 },
        { duration: '1m', target: 3 },
        { duration: '1m', target: 0 },
      ],
      exec: 'resourceManager',
    },
    // Engineer 2: Creating and planning maintenances
    maintenance_planner: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 2 },
        { duration: '2m', target: 5 },
        { duration: '1m', target: 3 },
        { duration: '1m', target: 0 },
      ],
      exec: 'maintenancePlanner',
      startTime: '15s', // Start slightly later
    },
    // Engineer 3: Executing maintenance workflow
    maintenance_executor: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 2 },
        { duration: '2m', target: 4 },
        { duration: '1m', target: 2 },
        { duration: '1m', target: 0 },
      ],
      exec: 'maintenanceExecutor',
      startTime: '30s', // Start even later
    },
    // Engineer 4: Monitoring and viewing calendar
    calendar_viewer: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 5 },
        { duration: '2m', target: 10 },
        { duration: '1m', target: 5 },
        { duration: '1m', target: 0 },
      ],
      exec: 'calendarViewer',
      startTime: '10s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_failed: ['rate<0.1'], // Allow 10% errors in complex scenario
    'http_req_duration{group:::Resource Management}': ['p(95)<500'],
    'http_req_duration{group:::Maintenance Planning}': ['p(95)<800'],
    'http_req_duration{group:::Maintenance Execution}': ['p(95)<1000'],
    'http_req_duration{group:::Calendar Viewing}': ['p(95)<500'],
  },
};

// Scenario 1: Resource Manager
// Creates resources, searches them, and checks types
export function resourceManager() {
  const headers = getHeaders();

  group('Resource Management', function() {
    // Create resource
    const createPayload = generateResourcePayload();
    const createRes = http.post(
      `${BASE_URL}/api/v1/resource/create`,
      JSON.stringify(createPayload),
      { headers }
    );

    const success = checkResponse(createRes, 200, 'RM: Create Resource');
    let resourceId = null;

    if (success && createRes.status === 200) {
      const body = parseResponse(createRes);
      if (body && body.id) {
        resourceId = body.id;
      }
    }

    sleep(sleepWithJitter(1, 0.5));

    // Search for resources
    const searchRes = http.get(
      `${BASE_URL}/api/v1/resources?name=${encodeURIComponent(createPayload.name.substring(0, 8))}`,
      { headers }
    );

    checkResponse(searchRes, 200, 'RM: Search Resources');

    sleep(sleepWithJitter(0.5, 0.3));

    // Get resource by id (the /types endpoint is gone: a resource is
    // maintained as a whole, without component types)
    if (resourceId) {
      const getRes = http.get(
        `${BASE_URL}/api/v1/resource/${resourceId}`,
        { headers }
      );

      checkResponse(getRes, 200, 'RM: Get Resource');
    }

    sleep(sleepWithJitter(2, 1));
  });
}

// Scenario 2: Maintenance Planner
// Creates maintenance drafts, edits them, and views details
export function maintenancePlanner() {
  const headers = getHeaders();

  group('Maintenance Planning', function() {
    // Scope the draft to its own fresh resource so it stays conflict-free
    const resourceId = createResource(BASE_URL, headers);
    if (!resourceId) {
      sleep(sleepWithJitter(1, 0.5));
      return;
    }

    // Create maintenance draft
    const createPayload = generateMaintenancePayload([resourceId]);
    const createRes = http.post(
      `${BASE_URL}/api/v1/maintenances/create`,
      JSON.stringify(createPayload),
      { headers }
    );

    const success = checkResponse(createRes, 200, 'MP: Create Draft');
    let maintenanceId = null;

    if (success && createRes.status === 200) {
      const body = parseResponse(createRes);
      if (body && body.id) {
        maintenanceId = body.id;
      }
    }

    if (!maintenanceId) {
      // Back off instead of hot-looping when creation keeps failing
      sleep(sleepWithJitter(1, 0.5));
      return;
    }

    sleep(sleepWithJitter(1, 0.5));

    // Get maintenance view to check conflicts
    const viewRes = http.get(
      `${BASE_URL}/ui/v1/maintenances/${maintenanceId}`,
      { headers }
    );

    checkResponse(viewRes, 200, 'MP: Get View');

    sleep(sleepWithJitter(1, 0.5));

    // Update the draft — keep it scoped to the same fresh resource
    const updatePayload = generateMaintenancePayload([resourceId]);
    updatePayload.title = `Updated ${createPayload.title}`;

    const updateRes = http.post(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}/edit`,
      JSON.stringify(updatePayload),
      { headers }
    );

    checkResponse(updateRes, 204, 'MP: Update Draft');

    sleep(sleepWithJitter(0.5, 0.3));

    // Get updated maintenance
    const getRes = http.get(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
      { headers }
    );

    checkResponse(getRes, 200, 'MP: Get Updated');

    sleep(sleepWithJitter(2, 1));
  });
}

// Scenario 3: Maintenance Executor
// Runs full maintenance lifecycle: create -> approve -> start -> complete/cancel
export function maintenanceExecutor() {
  const headers = getHeaders();

  group('Maintenance Execution', function() {
    // Scope the maintenance to its own fresh resource so it stays conflict-free
    const resourceId = createResource(BASE_URL, headers);
    if (!resourceId) {
      sleep(sleepWithJitter(1, 0.5));
      return;
    }

    // Create maintenance
    const createPayload = generateMaintenancePayload([resourceId]);
    const createRes = http.post(
      `${BASE_URL}/api/v1/maintenances/create`,
      JSON.stringify(createPayload),
      { headers }
    );

    const success = checkResponse(createRes, 200, 'ME: Create');
    let maintenanceId = null;

    if (success && createRes.status === 200) {
      const body = parseResponse(createRes);
      if (body && body.id) {
        maintenanceId = body.id;
      }
    }

    if (!maintenanceId) {
      // Back off instead of hot-looping when creation keeps failing
      sleep(sleepWithJitter(1, 0.5));
      return;
    }

    sleep(sleepWithJitter(1, 0.5));

    // Approve requires the current observed_maint_revision plus the
    // conflicts_snapshot the server previewed. Because the maintenance is
    // scoped to its own fresh resource, that snapshot is reliably empty, so a
    // single read-then-approve is enough — no drift-under-load 409s to retry.
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

    const approveRes = http.post(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}/approve`,
      JSON.stringify({
        observed_maint_revision: revision,
        conflicts_snapshot: conflicts
      }),
      { headers }
    );

    const approveSuccess = checkResponse(approveRes, 204, 'ME: Approve');

    // Only proceed if approve was successful
    if (!approveSuccess || approveRes.status !== 204) {
      return; // Cannot proceed without approval
    }

    sleep(sleepWithJitter(1, 0.5));

    // Randomly decide to complete or cancel
    const shouldCancel = Math.random() > 0.7; // 30% chance to cancel

    if (shouldCancel) {
      // Cancel maintenance
      const cancelPayload = {
        reason: randomItem(CANCEL_REASONS),
        comment: 'Load test - simulated cancellation'
      };

      const cancelRes = http.post(
        `${BASE_URL}/api/v1/maintenances/${maintenanceId}/cancel`,
        JSON.stringify(cancelPayload),
        { headers }
      );

      checkResponse(cancelRes, 204, 'ME: Cancel');
    } else {
      // Start maintenance
      const startRes = http.post(
        `${BASE_URL}/api/v1/maintenances/${maintenanceId}/start`,
        null,
        { headers }
      );

      const startSuccess = checkResponse(startRes, 204, 'ME: Start');

      // Only complete if start was successful
      if (!startSuccess || startRes.status !== 204) {
        return; // Cannot complete without starting
      }

      sleep(sleepWithJitter(2, 1)); // Simulate work being done

      // Complete every step first — the API rejects completing a
      // maintenance while any step is still pending
      const stepsRes = http.get(
        `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
        { headers }
      );

      const stepsBody = parseResponse(stepsRes);
      const steps = (stepsBody && stepsBody.steps) || [];
      for (const step of steps) {
        const stepStartRes = http.post(
          `${BASE_URL}/api/v1/maintenances/${maintenanceId}/steps/${step.id}/start`,
          null,
          { headers }
        );
        checkResponse(stepStartRes, 204, 'ME: Step Start');

        sleep(sleepWithJitter(0.5, 0.3)); // Simulate step work

        const stepCompleteRes = http.post(
          `${BASE_URL}/api/v1/maintenances/${maintenanceId}/steps/${step.id}/complete`,
          null,
          { headers }
        );
        checkResponse(stepCompleteRes, 204, 'ME: Step Complete');
      }

      // Complete maintenance
      const completeRes = http.post(
        `${BASE_URL}/api/v1/maintenances/${maintenanceId}/complete`,
        null,
        { headers }
      );

      checkResponse(completeRes, 204, 'ME: Complete');
    }

    sleep(sleepWithJitter(1, 0.5));

    // Verify final state
    const finalRes = http.get(
      `${BASE_URL}/api/v1/maintenances/${maintenanceId}`,
      { headers }
    );

    checkResponse(finalRes, 200, 'ME: Verify Final');

    sleep(sleepWithJitter(2, 1));
  });
}

// Scenario 4: Calendar Viewer
// Continuously monitors the calendar and checks maintenance details
export function calendarViewer() {
  const headers = getHeaders();

  group('Calendar Viewing', function() {
    // View calendar
    const dateRange = generateDateRange(7, 30);
    const calendarRes = http.get(
      `${BASE_URL}/ui/v1/calendar?from=${dateRange.from}&to=${dateRange.to}`,
      { headers }
    );

    const success = checkResponse(calendarRes, 200, 'CV: Calendar');
    check(calendarRes, {
      'CV: has events': (r) => {
        const body = parseResponse(r);
        return body && Array.isArray(body.events);
      }
    });

    // If there are events, view details of a random one
    if (success && calendarRes.status === 200) {
      const body = parseResponse(calendarRes);
      if (body && body.events && body.events.length > 0) {
        const randomEvent = randomItem(body.events);

        sleep(sleepWithJitter(0.5, 0.3));

        const detailRes = http.get(
          `${BASE_URL}/ui/v1/maintenances/${randomEvent.id}`,
          { headers }
        );

        checkResponse(detailRes, 200, 'CV: Event Details');
      }
    }

    sleep(sleepWithJitter(2, 1));

    // View calendar with different date range
    const shortRange = generateDateRange(0, 7);
    const shortCalendarRes = http.get(
      `${BASE_URL}/ui/v1/calendar?from=${shortRange.from}&to=${shortRange.to}&statuses=draft&statuses=planned`,
      { headers }
    );

    checkResponse(shortCalendarRes, 200, 'CV: Filtered Calendar');

    sleep(sleepWithJitter(3, 1));
  });
}
