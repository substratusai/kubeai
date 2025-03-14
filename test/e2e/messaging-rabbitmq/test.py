#!/usr/bin/env python3

import json
import os
import time
import pika

def send_request(channel, request_data):
    """Send a request message to RabbitMQ."""
    channel.basic_publish(
        exchange='requests',
        routing_key='',  # Fanout exchange ignores routing key
        body=json.dumps(request_data)
    )

def receive_response(channel):
    """Receive and process response from RabbitMQ."""
    method_frame, header_frame, body = channel.basic_get(queue=responses_queue)
    if method_frame:
        channel.basic_ack(method_frame.delivery_tag)
        return json.loads(body)
    return None

def main():
    # Connect to RabbitMQ
    # Connect to RabbitMQ using port-forwarded address
    connection = pika.BlockingConnection(pika.ConnectionParameters(
        host='localhost',
        port=5672,
        credentials=pika.PlainCredentials('guest', 'guest')
    ))
    channel = connection.channel()

    # Declare exchanges and queues
    channel.exchange_declare(exchange='requests', exchange_type='fanout')
    channel.exchange_declare(exchange='responses', exchange_type='fanout')
    
    # Declare queues
    result = channel.queue_declare(queue='', exclusive=True)
    requests_queue = result.method.queue
    result = channel.queue_declare(queue='', exclusive=True)
    responses_queue = result.method.queue
    
    # Bind queues to exchanges
    channel.queue_bind(exchange='requests', queue=requests_queue)
    channel.queue_bind(exchange='responses', queue=responses_queue)

    # Test chat completion requests with different ToolChoice values
    tool_choice_values = ["none", "auto", "required"]
    
    for i, tool_choice in enumerate(tool_choice_values):
        request_data = {
            "path": "/v1/chat/completions",
            "body": {
                "model": "opt-125m-cpu",
                "messages": [{"role": "user", "content": "Hello"}],
                "tool_choice": tool_choice
            },
            "metadata": {"request_id": f"test-tool-choice-{tool_choice}"}
        }

        print(f"Sending chat completion request with tool_choice={tool_choice}...")
        send_request(channel, request_data)

        # Wait for and verify response
        max_retries = 10
        retry_count = 0
        response = None

        while retry_count < max_retries:
            response = receive_response(channel)
            if response:
                break
            time.sleep(1)
            retry_count += 1

        if not response:
            raise Exception(f"No response received after {max_retries} retries")

        # Verify response structure
        assert response["status_code"] == 200, f"Expected status 200, got {response['status_code']}"
        assert "body" in response, "Response missing body"
        assert "choices" in response["body"], "Response missing choices"
        assert response["metadata"]["request_id"] == f"test-tool-choice-{tool_choice}", "Response metadata mismatch"

        print(f"Chat completion test with tool_choice={tool_choice} successful!")

    # Test with a specific tool choice object
    request_data = {
        "path": "/v1/chat/completions",
        "body": {
            "model": "opt-125m-cpu",
            "messages": [{"role": "user", "content": "Hello"}],
            "tool_choice": {"type": "function", "function": {"name": "test_function"}}
        },
        "metadata": {"request_id": "test-tool-choice-object"}
    }

    print("Sending chat completion request with tool_choice object...")
    send_request(channel, request_data)

    # Wait for and verify response
    retry_count = 0
    response = None

    while retry_count < max_retries:
        response = receive_response(channel)
        if response:
            break
        time.sleep(1)
        retry_count += 1

    if not response:
        raise Exception(f"No response received after {max_retries} retries")

    # Verify response structure
    assert response["status_code"] == 200, f"Expected status 200, got {response['status_code']}"
    assert "body" in response, "Response missing body"
    assert "choices" in response["body"], "Response missing choices"
    assert response["metadata"]["request_id"] == "test-tool-choice-object", "Response metadata mismatch"

    print("Chat completion test with tool_choice object successful!")

    # Test error case with non-existent model
    error_request = {
        "path": "/v1/chat/completions",
        "body": {
            "model": "non-existent-model",
            "messages": [{"role": "user", "content": "Hello"}]
        },
        "metadata": {"request_id": "test-error"}
    }

    print("Testing error case with non-existent model...")
    send_request(channel, error_request)

    # Wait for and verify error response
    retry_count = 0
    response = None

    while retry_count < max_retries:
        response = receive_response(channel)
        if response:
            break
        time.sleep(1)
        retry_count += 1

    if not response:
        raise Exception("No error response received after {} retries".format(max_retries))

    assert response["status_code"] == 404, f"Expected status 404, got {response['status_code']}"
    assert "error" in response["body"], "Error response missing error field"
    assert response["metadata"]["request_id"] == "test-error", "Error response metadata mismatch"

    print("Error case test successful!")

    # Clean up
    channel.queue_unbind(exchange='requests', queue=requests_queue)
    channel.queue_unbind(exchange='responses', queue=responses_queue)
    channel.queue_delete(queue=requests_queue)
    channel.queue_delete(queue=responses_queue)
    channel.exchange_delete(exchange='requests')
    channel.exchange_delete(exchange='responses')
    connection.close()

if __name__ == "__main__":
    main()
