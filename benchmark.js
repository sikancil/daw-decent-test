import http from 'k6/http';
import { check, group } from 'k6';

// Configuration: 50 concurrent users constantly hammering the API for 10 seconds
export const options = {
  vus: 50,
  duration: '10s',
  thresholds: {
    http_req_duration: ['p(95)<50', 'p(99)<100'], // 95% of requests must be < 50ms
    http_req_failed: ['rate<0.01'],               // Error rate must be < 1%
  },
};

export default function () {
  const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';
  const headers = { 'Content-Type': 'application/json' };

  // FIXED: Use unique identifier per Virtual User
  // __VU is the Virtual User ID (0-49), __ITER is the iteration number
  // This ensures each VU operates on its own data, eliminating race conditions
  const vuId = __VU;
  const uniqueSuffix = `vu${vuId}-iter${__ITER}`;

  group('Level 1 & 2: Ping & Echo', () => {
    check(http.get(`${BASE_URL}/ping`), { 'Ping is 200': (r) => r.status === 200 });

    const echoRes = http.post(`${BASE_URL}/echo`, JSON.stringify({
      hello: "world",
      from: `vu${vuId}`
    }), { headers });
    check(echoRes, { 'Echo is 200': (r) => r.status === 200 });
  });

  group('Level 5: Auth Guard', () => {
    const authRes = http.post(`${BASE_URL}/auth/token`);
    check(authRes, { 'Auth is 200': (r) => r.status === 200 });
    // Extract token for subsequent requests
    const token = authRes.json('token');
    headers['Authorization'] = `Bearer ${token}`;
  });

  // FIXED: Each VU creates and operates on its own unique book
  let bookId = '';

  group('Level 3: Create & Read', () => {
    const createRes = http.post(`${BASE_URL}/books`, JSON.stringify({
      title: `The Art of War (${uniqueSuffix})`,
      author: `Sun Tzu (${uniqueSuffix})`,
      year: 2026
    }), { headers });

    check(createRes, { 'Create is 201': (r) => r.status === 201 });
    bookId = createRes.json('id');

    check(http.get(`${BASE_URL}/books`, { headers }), { 'Read All is 200': (r) => r.status === 200 });
    check(http.get(`${BASE_URL}/books/${bookId}`, { headers }), { 'Read Single is 200': (r) => r.status === 200 });
  });

  group('Level 6: Search & Paginate', () => {
    // Search for this VU's specific author
    check(http.get(`${BASE_URL}/books?author=vu${vuId}&page=1&limit=10`, { headers }), {
      'Search & Paginate is 200': (r) => r.status === 200
    });
  });

  group('Level 4: Update & Delete', () => {
    check(http.put(`${BASE_URL}/books/${bookId}`, JSON.stringify({
      title: `The Art of War - Updated (${uniqueSuffix})`,
      author: `Sun Tzu (${uniqueSuffix})`,
      year: 2026
    }), { headers }), { 'Update is 200': (r) => r.status === 200 });

    check(http.del(`${BASE_URL}/books/${bookId}`, null, { headers }), {
      'Delete is 204': (r) => r.status === 204
    });
  });

  group('Level 7: Error Handling', () => {
    check(http.post(`${BASE_URL}/books`, JSON.stringify({ title: "No Author" }), { headers }), {
      'Invalid Create is 400': (r) => r.status === 400
    });

    check(http.get(`${BASE_URL}/books/invalid-uuid-${uniqueSuffix}`, { headers }), {
      'Not Found is 404': (r) => r.status === 404
    });
  });
}
