"""
Generate a JSONL file with request body for OpenAI API
"""
import json
import requests

model = "mistral-7b-instruct-v0.1"

test_data = []

url = "https://the-trivia-api.com/v2/questions"
params = {'limit': 50}
questions = requests.get(url, params=params).json()
for question in questions:
    question = question["question"]["text"]
    test_data.append({
        "model": model,
        "prompt": question,
        "max_tokens": 50,
    })


with open("test_data.jsonl", "a") as f:
    for data in test_data:
        json.dump(data, f)
        f.write("\n")
