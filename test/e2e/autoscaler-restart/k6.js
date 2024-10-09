import http from 'k6/http';

export const options = {
  stages: [
    { duration: '10m', target: 3 },
  ],
};

export default function () {
  const url = 'http://kubeai/openai/v1/completions';

  let data = {
    "prompt": "Your text string goes here",
    "model": "opt-125m-cpu"
  };

  let res = http.post(url, JSON.stringify(data), {
    headers: { 'Content-Type': 'application/json' },
  });
}
