"""
Batch inference using asyncio.Queue
"""

import argparse
import asyncio
import json

import aiohttp
from smart_open import open

url = "http://localhost:8080/v1/completions"
filename = "part-{partition}.jsonl"
concurrent_requests = 100
requests_path = ""
output_path = "/tmp/lingo-batch-inference"
flush_every = 1000


async def read_file_and_enqueue(path, queue: asyncio.Queue):
    with open(path, mode="r") as file:
        print(f"Sending request to Queue from file {path}")
        for line in file.readlines():
            request = json.loads(line)
            await queue.put(request)
    await queue.put(None)


async def worker(
    requests: asyncio.Queue,
    results: asyncio.Queue,
    session: aiohttp.ClientSession,
    worker_id: int,
    url: str,
):
    print(f"Starting worker {worker_id}")
    while True:
        request = await requests.get()
        print(f"{worker_id}: Got request {request}")
        if request is None:
            requests.task_done()
            break
        async with session.post(url=url, json=request) as response:
            print(f"{worker_id}: HTTP {response.status}")
            response = await response.json()
            await results.put({"request": request, "response": response})
            requests.task_done()


async def flusher(results: asyncio.Queue, flush_every: int, output_path: str):
    print("Starting flusher")
    current_batch = []
    partition = 1
    while True:
        result = await results.get()
        if result is None and len(current_batch) == 0:
            results.task_done()
            break
        current_batch.append(result)
        if len(current_batch) >= flush_every or result is None:
            if result is None:
                current_batch.pop()
                print(f"Flushing last batch of {len(current_batch) - 1} results")
            else:
                print(f"Flushing batch of {len(current_batch)} results")
            jsonl_data = "\n".join(json.dumps(entry) for entry in current_batch)
            partitioned_filename = (
                output_path + "/" + filename.format(partition=partition)
            )
            with open(partitioned_filename, mode="w") as file:
                file.write(jsonl_data)
            for _ in range(0, len(current_batch)):
                results.task_done()
            current_batch = []
            partition += 1
            if result is None:
                break


async def main():
    requests = asyncio.Queue(maxsize=concurrent_requests)
    results = asyncio.Queue(maxsize=flush_every)
    timeout = aiohttp.ClientTimeout(total=600)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        producer_task = asyncio.create_task(
            read_file_and_enqueue(requests_path, requests)
        )
        flusher_task = asyncio.create_task(flusher(results, flush_every, output_path))
        workers = [
            asyncio.create_task(worker(requests, results, session, worker_id, url))
            for worker_id in range(concurrent_requests)
        ]
        await producer_task
        print("Finished reading requests file and enqueueing requests")
        print("Waiting for all requests to be processed")
        await requests.join()
        print("All requests have been processed")
        # Send a signal that all requests have been processed
        await results.put(None)
        print("Waiting for all results to be flushed")
        await flusher_task
        for w in workers:
            w.cancel()
        # await asyncio.gather(*workers, return_exceptions=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Test Lingo using Python OpenAI API")
    parser.add_argument("--url", type=str, default=url)
    parser.add_argument("--requests-path", type=str)
    parser.add_argument("--output-path", type=str)
    parser.add_argument("--flush-every", type=int, default=flush_every)
    args = parser.parse_args()
    requests_path = args.requests_path
    output_path = args.output_path
    flush_every = args.flush_every
    url = args.url
    asyncio.run(main())
