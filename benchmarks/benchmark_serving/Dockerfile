# Use a lightweight Python base image
FROM python:3.10

# Set the working directory
WORKDIR /app

# Copy requirements first to leverage Docker cache
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy the benchmark serving script
COPY sharegpt_16_messages_or_more.json .
COPY backend_request_func.py .
COPY benchmark_serving.py .

# Set environment variables
ENV PYTHONPATH=/app

# Define the entrypoint command
ENTRYPOINT ["python", "benchmark_serving.py"]

CMD ["--dataset-name=sharegpt", "--dataset-path=sharegpt_16_messages_or_more.json"]
