import argparse
import concurrent.futures
from openai import OpenAI

parser = argparse.ArgumentParser(description="Test Lingo using Python OpenAI API")
parser.add_argument("--base-url", type=str, default="http://localhost:8080/v1")
parser.add_argument("--requests", type=int, default=60)
parser.add_argument("--model", type=str, default="text-embedding-ada-002")
parser.add_argument("--text", type=str, default="Generate an embedding for me")
parser.add_argument("--client-per-thread", type=bool, default=False)
args = parser.parse_args()

def create_client():
    return OpenAI(
        api_key="this won't be used",
        base_url=args.base_url,
    )

client = create_client()

def embedding_request(index: int):
    print (f"Request {index} of {args.requests}")
    if args.client_per_thread:
        client = create_client()
    embedding = client.embeddings.create(model=args.model, input=args.text)
    print (f"Finished {index} of {args.requests}")
    return embedding

with concurrent.futures.ThreadPoolExecutor(max_workers=args.requests) as executor:
    futures = [executor.submit(embedding_request, i+1) for i in range(args.requests)]
    results = [future.result() for future in concurrent.futures.as_completed(futures, timeout=600)]
    assert len(results) == args.requests
