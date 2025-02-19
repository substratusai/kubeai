from fastapi import FastAPI
from pydantic import BaseModel
from transformers import AutoTokenizer
import os

app = FastAPI()
tokenizer_model = os.environ["TOKENIZER_MODEL"]
print("Tokenizer model:", tokenizer_model)
# TODO: Account for model_max_length
tokenizer = AutoTokenizer.from_pretrained(tokenizer_model)

print(len(tokenizer("Your code appears to be a web application built using").input_ids))


class TextInput(BaseModel):
    text: str


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/tokens")
def count_tokens(data: TextInput):
    # Tokenize text
    input_ids = tokenizer(data.text).input_ids
    # Count the number of tokens
    num_tokens = len(input_ids)
    return {"num_tokens": num_tokens}
