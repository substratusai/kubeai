# Tokenizer

```bash
python -m venv .venv
source .venv/bin/activate
pip install pydantic 'fastapi[standard]' transformers
```

```bash
TOKENIZER_MODEL=gpt2 ./.venv/bin/fastapi run tokens.py --port 7000
```