# OpenAI API

The goal of these API structs is to define a standard API for interfacing with models using OpenAI-compatible APIs.

* JSON-to-struct-to-JSON round trip is not guaranteed. This package strives to preserve semantic meaning during JSON round trips.
* Use the `easyjson` pkg, not the stdlib `encoding/json` pkg.
* Extra fields at the root of requests/responses should be preserved (supports additional fields that engines like vLLM support - see [vLLM docs](https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html#extra-parameters)).

## Generation

To generate the `easyjson` implementations, run the following command:

```bash
make generate
```

**NOTE:** There are edge cases where you might need to comment out parts of the custom marshalling funcs and remove the generated files before `make generate` will succeed.

## API References

API References (useful for AI generation) were generated using:

```bash
wget https://raw.githubusercontent.com/openai/openai-openapi/refs/heads/master/openapi.yaml -O ./tmp/openaiapi.yaml

# Filter down to only the relevant components.
# This allows you to focus an AI coding assistant on
# a specific part of the API.
./hack/filter-openapi-components.py ./tmp/openaiapi.yaml /completions post -o ./api/openai/v1/completion.openai.openapi.yaml

./hack/filter-openapi-components.py ./tmp/openaiapi.yaml /completions post -o ./api/openai/v1/completion.openai.openapi.yaml
```

## Concerns

When developing, pay special attention to the following:

- Zero-value types missmatching with default values in the API.
- Pointer types and optional types.
- Nullable types.