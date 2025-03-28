# How to Configure Kafka Messaging for KubeAI

This guide explains how to configure KubeAI to use Apache Kafka as its messaging system for handling inference requests and responses.

## Prerequisites

*   A running Kafka cluster accessible from your Kubernetes cluster.
*   Two Kafka topics created: one for requests and one for responses.

## Configuration

To use Kafka, you need to configure the `messaging` section in your KubeAI Helm `values.yaml` file.

Here's an example configuration:

```yaml
messaging:
  # Optional: Maximum backoff duration for retrying failed message processing.
  errorMaxBackoff: 30s

  # Define one or more message streams.
  streams:
  - # URL for consuming requests. Format: kafka://<consumer-group>?topic=<request-topic>
    # Replace <consumer-group> with your desired Kafka consumer group ID.
    # Replace <request-topic> with the name of your Kafka topic for requests.
    requestsURL: "kafka://my-kubeai-group?topic=kubeai-requests"

    # URL for publishing responses. Format: kafka://<response-topic>
    # Replace <response-topic> with the name of your Kafka topic for responses.
    responsesURL: "kafka://kubeai-responses"

    # Optional: Maximum concurrent message handlers for this stream.
    maxHandlers: 5000

    # Optional: Kafka-specific configuration (authentication, TLS, etc.)
    # See gocloud.dev/pubsub/kafkapubsub documentation for details.
    # kafkaOptions:
    #   brokers: "kafka-broker1:9092,kafka-broker2:9092"
    #   saslUsername: "my-user"
    #   saslPassword: "my-password"
    #   saslMechanism: "PLAIN" # or SCRAM-SHA-256, SCRAM-SHA-512
    #   tlsEnable: true
    #   tlsCaCertPath: "/path/to/ca.crt"
    #   tlsClientCertPath: "/path/to/client.crt"
    #   tlsClientKeyPath: "/path/to/client.key"
    #   tlsInsecureSkipVerify: false
```

### Parameter Explanations

*   `messaging.errorMaxBackoff`: (Optional) Sets the maximum time KubeAI will wait before retrying to process a message after an error.
*   `messaging.streams`: An array defining different message streams KubeAI can handle.
*   `requestsURL`: The connection string for the request topic.
    *   `kafka://`: Specifies the Kafka protocol.
    *   `<consumer-group>`: Identifies the Kafka consumer group KubeAI will join to read requests. Using a unique group ensures messages are distributed among KubeAI instances if scaled.
    *   `?topic=<request-topic>`: Specifies the Kafka topic from which KubeAI will consume inference requests.
*   `responsesURL`: The connection string for the response topic.
    *   `kafka://`: Specifies the Kafka protocol.
    *   `<response-topic>`: Specifies the Kafka topic to which KubeAI will publish inference responses.
*   `maxHandlers`: (Optional) Controls the maximum number of concurrent requests KubeAI will process from this stream. Adjust based on your expected load and resource availability.
*   `kafkaOptions`: (Optional) Allows for advanced Kafka client configuration, including broker addresses, SASL authentication, and TLS encryption. Refer to the [Go CDK Kafka documentation](https://gocloud.dev/pubsub/kafkapubsub/) for all available options. *Note: You might need to mount certificates/keys as secrets into the KubeAI pod.*
