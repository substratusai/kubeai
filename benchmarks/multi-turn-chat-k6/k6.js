import { check } from 'k6';
import { scenario } from 'k6/execution';
import http from 'k6/http';
import { Trend, Counter } from 'k6/metrics';

const model_addr = __ENV.MODEL_ADDR;
const config_dir = __ENV.CONFIG_DIR;
const data_dir = __ENV.DATA_DIR;

const timePerToken = new Trend('time_per_token', true);
const tokens = new Counter('tokens');
const new_tokens = new Counter('new_tokens');
const input_tokens = new Counter('input_tokens');

const k6Options = JSON.parse(open(`${config_dir}/k6.json`));
const baseRequest = JSON.parse(open(`${config_dir}/base-request.json`));
const messageThreads = JSON.parse(open(`${data_dir}/message-threads.json`))

export const options = k6Options;

export default function run() {
    const headers = { 'Content-Type': 'application/json' };
    const msgThread = messageThreads[scenario.iterationInTest % messageThreads.length];
    var payload = JSON.parse(JSON.stringify(baseRequest));

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
