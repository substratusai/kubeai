import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  stages: [
    { duration: '15s', target: 1 },
    { duration: '15s', target: 9 },
    { duration: '1m', target: 9 },
    { duration: '15s', target: 0 },
    { duration: '15s', target: 0 },
  ],
};

export default function () {
  const url = 'http://kubeai/openai/v1/completions';

  let data = {
    "prompt": "Your text string goes here",
    "model": "dev"
  };

  let res = http.post(url, JSON.stringify(data), {
    headers: { 'Content-Type': 'application/json' },
  });

  sleep(1);
}
