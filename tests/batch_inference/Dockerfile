FROM python:3.11-slim

ENV REQUESTS_PATH=gs://mybucket/requests.jsonl
ENV OUTPUT_PATH=gs://mybucket/output.jsonl/
ENV FLUSH_EVERY=100
ENV URL=http://localhost:8080/v1/completions

# Set the working directory in the container
WORKDIR /usr/src/app

COPY main.py /usr/src/app
COPY requirements.txt /usr/src/app

# Install any needed packages specified in requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Run the Python script when the container launches
CMD python main.py --requests-path $REQUESTS_PATH \
      --output-path $OUTPUT_PATH \
      --flush-every $FLUSH_EVERY \
      --url $URL