import asyncio
import pathlib
import json
import pytest
import os
import unittest
from unittest.mock import patch, mock_open

from .main import flusher

@pytest.mark.asyncio
async def test_batching_and_flushing(tmp_path: pathlib.Path):
    results = asyncio.Queue(maxsize=100)
    flush_every = 10
    test_data = [{"title": f"result_{i}"} for i in range(25)]
    for data in test_data:
        await results.put(data)
    await results.put(None)  # Signal to end the loop

    await flusher(results, flush_every, output_path=str(tmp_path))
    assert len(os.listdir(tmp_path)) == 3
    # Ensure the total lines of the 3 files in tmp_path is equal to 25
    total_lines = 0
    for file in os.listdir(tmp_path):
        with open(tmp_path / file, mode="r") as f:
            print(f"Contents of file {file}: {f.read()}")
            f.seek(0)
            total_lines += len(f.readlines())

    assert total_lines == 25, f"Expected 25 lines, got {total_lines}."



@pytest.mark.asyncio
async def test_termination_condition():
    results = asyncio.Queue(maxsize=100)
    await results.put(None)  # Signal to end the loop
    await flusher(results, flush_every=10, output_path="")

@pytest.mark.asyncio
async def test_worker():
    requests = asyncio.Queue(maxsize=100)
    results = asyncio.Queue(maxsize=100)
    worker_id = 1
    url = "http://localhost:8080/v1/inference"
    session = aiohttp.ClientSession()
    await requests.put({"title": "result_1"})
    await requests.put(None)

