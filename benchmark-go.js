import http from 'k6/http';
import { check, group } from 'k6';

export const options = {
  vus: 50,
  duration: '10s',
  thresholds: {
    http_req_duration: ['p(95)<50', 'p(99)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const BASE_URL = __ENV.BASE_URL || 'http://localhost:3081';
  const headers = { 'Content-Type': 'application/json' };
  const vuId = __VU;
  const iter = __ITER;
  const STACK_ID = 'go'; // Stack-specific identifier

  group('Level 1 & 2: Ping & Echo', () => {
    check(http.get(`${BASE_URL}/ping`), { 'Ping is 200': (r) => r.status === 200 });
    check(http.post(`${BASE_URL}/echo`, JSON.stringify({ 
      hello: "world", 
      stack: STACK_ID,
      vu: vuId 
    }), { headers }), { 'Echo is 200': (r) => r.status === 200 });
  });

  group('Level 5: Auth Guard', () => {
    const authRes = http.post(`${BASE_URL}/auth/token`);
    check(authRes, { 'Auth is 200': (r) => r.status === 200 });
    const token = authRes.json('token');
    headers['Authorization'] = `Bearer ${token}`;
  });

  let bookId = '';

  group('Level 3: Create & Read', () => {
    const createRes = http.post(`${BASE_URL}/books`, JSON.stringify({
      title: `${STACK_ID}-vu${vuId}-iter${iter}`,
      author: `${STACK_ID}-Author-vu${vuId}`,
      year: 2026
    }), { headers });

    check(createRes, { 'Create is 201': (r) => r.status === 201 });
    bookId = createRes.json('id');

    check(http.get(`${BASE_URL}/books`, { headers }), { 'Read All is 200': (r) => r.status === 200 });
    check(http.get(`${BASE_URL}/books/${bookId}`, { headers }), { 'Read Single is 200': (r) => r.status === 200 });
  });

  group('Level 6: Search & Paginate', () => {
    check(http.get(`${BASE_URL}/books?author=${STACK_ID}-Author-vu${vuId}&page=1&limit=10`, { headers }), {
      'Search & Paginate is 200': (r) => r.status === 200
    });
  });

  group('Level 4: Update & Delete', () => {
    check(http.put(`${BASE_URL}/books/${bookId}`, JSON.stringify({
      title: `${STACK_ID}-vu${vuId}-iter${iter}-updated`,
      author: `${STACK_ID}-Author-vu${vuId}`,
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

    check(http.get(`${BASE_URL}/books/${STACK_ID}-nonexistent-${vuId}-${iter}`, { headers }), {
      'Not Found is 404': (r) => r.status === 404
    });
  });
}
