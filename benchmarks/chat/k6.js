import { check } from 'k6';
import { scenario } from 'k6/execution';
import http from 'k6/http';
import { Trend, Counter } from 'k6/metrics';

const model_addr = __ENV.MODEL_ADDR;
const model_id = __ENV.MODEL_ID;
const timePerToken = new Trend('time_per_token', true);
const tokens = new Counter('tokens');
const new_tokens = new Counter('new_tokens');
const input_tokens = new Counter('input_tokens');
const max_new_tokens = 50;

const messageThreads = JSON.parse(open("message-threads.json"))

export const options = {
    thresholds: {
        http_req_failed: ['rate==0'],
    },
    scenarios: {
        chat: {
            executor: 'shared-iterations',
            // Number of VUs to run concurrently.
            vus: 20,
            // Total number of script iterations to execute across all VUs (b/c using 'shared-iterations' executor).
            iterations: 200,
            maxDuration: '120s',
        },
    },
};

export default function run() {
    const headers = { 'Content-Type': 'application/json' };
    const msgThread = messageThreads[scenario.iterationInTest % messageThreads.length];
    var payload = {
        "messages": [],
        "temperature": 0,
        "model": `${model_id}`,
        "max_tokens": max_new_tokens
    };

    // console.log(`Message thread: ${JSON.stringify(msgThread)}`);

    // Iterate over all the messages in the thread, appending the completions to the same payload.
    for (let i = 0; i < msgThread["userMessages"].length; i++) {
        payload.messages.push({
            "role": "user",
            "content": msgThread["userMessages"][i]
        });
        //console.log(`Payload: ${JSON.stringify(payload)}`);

        const res = http.post(`http://${model_addr}/v1/chat/completions`, JSON.stringify(payload), {
            headers,
        });
        if (res.status >= 400 && res.status < 500) {
            return;
        }

        check(res, {
            'Post status is 200': (res) => res.status === 200,
        });
        const duration = res.timings.duration;

        if (res.status === 200) {
            // console.log(`Status: ${res.status}`);
            const body = res.json();

            const completion_tokens = body.usage.completion_tokens;
            const prompt_tokens = body.usage.prompt_tokens;
            const latency_ms_per_token = duration / completion_tokens;

            new_tokens.add(completion_tokens);
            input_tokens.add(prompt_tokens);
            timePerToken.add(latency_ms_per_token);
            tokens.add(completion_tokens + prompt_tokens);

            const msg0 = body.choices[0].message;
            payload.messages.push({
                "role": msg0.role,
                "content": msg0.content
            });
        } else {
            console.log(`Error Status: ${res.status}`);
            console.log(`Response: ${res.body}`);
        }
    }
}
