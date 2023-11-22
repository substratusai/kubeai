"""
Batch inference using asyncio.Queue
"""

import argparse
import asyncio
import json

import aiohttp
from smart_open import open

url = "http://localhost:8080/v1/completions"

concurrent_requests = 100

async def read_file_and_enqueue(path, queue: asyncio.Queue):
    with open(path, mode='r') as file:
        request = json.loads(file.readline())
        await queue.put(request)
    await queue.put(None)

async def worker(queue: asyncio.Queue, session: aiohttp.ClientSession):
    while True:
        request = await queue.get()
        if request is None:
            break
        request_id = request.pop('id', "no_id")
        async with session.post(url=url, json=request) as response:
            print(f"{request_id} - HTTP {response.status}")
        queue.task_done()

async def main(path):
    queue = asyncio.Queue(maxsize=concurrent_requests)
    async with aiohttp.ClientSession(timeout=600) as session:
        file_task = asyncio.create_task(read_file_and_enqueue(path, queue))
        workers = [asyncio.create_task(worker(queue, session)) for _ in range(concurrent_requests)]
        await file_task
        await queue.join()
        for w in workers:
            w.cancel()
        # await asyncio.gather(*workers, return_exceptions=True)

if __name__ == '__main__':
    path = 'your_file_path_here.txt'
    asyncio.run(main(path))