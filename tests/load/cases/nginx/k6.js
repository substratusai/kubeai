// This nginx test is intended to establish a baseline for the performance of lingo
// itself (i.e. with a fast backend).
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  // Ramp up concurrent users over time:
  //
  //           _100______100_
  //          /              \
  //  _1_____/                \_0_____0__
  //
  stages: [
    { duration: '1m', target: 1 },
    { duration: '1m', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '1m', target: 0 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    // HTTP errors should be less than 1%.
    http_req_failed: ['rate < 0.01'],
    // 90% of http request durations should be under 3 seconds.
    // Using the p(90) metric should exclude the scale-up requests.
    http_req_duration: ['p(90) < 3000'],
    // All code-defined checks should pass.
    checks: ['rate == 1.0'],
  },
};

http.setResponseCallback(
  // NGINX returns 405 for POST requests by default.
  http.expectedStatuses(405)
);

export default function () {
  const url = 'http://lingo/';

  let data = {
    "input": "Your text string goes here",
    "model": "nginx"
  };

  let res = http.post(url, JSON.stringify(data), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'reached backend': (r) => r.headers['Server'] == 'nginx/1.25.3',
  });

  sleep(1);
}
