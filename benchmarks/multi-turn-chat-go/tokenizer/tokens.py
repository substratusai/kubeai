from fastapi import FastAPI
from pydantic import BaseModel
from transformers import AutoTokenizer
import os

app = FastAPI()
tokenizer = AutoTokenizer.from_pretrained(os.environ["TOKENIZER_MODEL"])


class TextInput(BaseModel):
    text: str


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/tokens")
def count_tokens(data: TextInput):
    # Tokenize text
    tokens = tokenizer(data.text)
    # Count the number of tokens
    num_tokens = len(tokens["input_ids"])
    return {"num_tokens": num_tokens}
