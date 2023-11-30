import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
    thresholds: {
        http_req_failed: ['rate<0.01'], // http errors should be less than 1%
    },
    vus: 3,
    duration: '1m',
};

export default function () {
    const url = 'http://lingo';

    let data = {
        "input": "Your text string goes here",
        "model": "perf-test"
    };

    http.post(url, JSON.stringify(data), {
        headers: { 'Content-Type': 'application/json' },
    });

    sleep(1);
}
