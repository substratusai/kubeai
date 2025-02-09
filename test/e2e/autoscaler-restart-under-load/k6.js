import http from 'k6/http';

export const options = {
  discardResponseBodies: true,
  scenarios: {
    contacts: {
      executor: 'constant-vus',
      vus: 3,
      duration: '10m',
    },
  },
};

export default function () {
  const url = 'http://kubeai/openai/v1/completions';

  let data = {
    "prompt": "Your text string goes here",
    "model": "opt-125m-cpu"
  };

  let res = http.post(url, JSON.stringify(data), {
    headers: { 'Content-Type': 'application/json' },
    timeout: "5s",
  });
}
