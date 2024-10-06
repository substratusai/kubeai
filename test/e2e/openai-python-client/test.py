from openai import OpenAI
import pytest

base_url = "http://localhost:8000/openai/v1"
model = "facebook-opt-125m"
client = OpenAI(base_url=base_url, api_key="ignored-by-kubeai")


def test_list_models():
    response = client.models.list()
    model_ids = [model["id"] for model in response["data"]]
    assert "facebook-opt-125m" in model_ids


# TODO: FIX: This test is failing b/c the model does not have a chat template:
# E           openai.BadRequestError: Error code: 400 - {'object': 'error', 'message': 'As of transformers v4.44, default chat template is no longer allowed, so you must provide a chat template if the tokenizer does not define one.', 'type': 'BadRequestError', 'param': None, 'code': 400}
def test_chat_completion():
    # Define the test parameters
    model = "opt-125m-cpu"
    messages = [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello, how are you?"},
    ]

    response = client.chat.completions.create(model=model, messages=messages)

    # Assert that the response contains the 'choices' key
    assert "choices" in response, "Response should contain 'choices' key"
