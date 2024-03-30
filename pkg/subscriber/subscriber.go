package subscriber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/gcppubsub"
)

type Subscriber struct {
	Deployments DeploymentManager
	Endpoints   EndpointManager
	Queues      QueueManager

	HTTPC *http.Client

	requests  *pubsub.Subscription
	responses *pubsub.Topic
}

func NewSubscriber(
	ctx context.Context,
	requestsURL string,
	responsesURL string,
	deployments DeploymentManager,
	endpoints EndpointManager,
	queues QueueManager,
	httpClient *http.Client,
) (*Subscriber, error) {

	// Example URL for GCP PubSub:
	// "gcppubsub://projects/my-project/subscriptions/my-subscription"
	requests, err := pubsub.OpenSubscription(ctx, requestsURL)
	if err != nil {
		return nil, err
	}

	responses, err := pubsub.OpenTopic(ctx, responsesURL)
	if err != nil {
		return nil, err
	}

	return &Subscriber{
		Deployments: deployments,
		Endpoints:   endpoints,
		Queues:      queues,
		HTTPC:       httpClient,
		requests:    requests,
		responses:   responses,
	}, nil
}

func (s *Subscriber) Start(ctx context.Context) error {
	for {
		msg, err := s.requests.Receive(ctx)
		if err != nil {
			return err
		}

		log.Println("Received message:", msg.LoggableID)

		if err := s.handleRequest(context.Background(), msg); err != nil {
			log.Printf("Error handling message %s: %v", msg.LoggableID, err)
			return err
		}
	}
}

func (s *Subscriber) handleRequest(ctx context.Context, msg *pubsub.Message) error {
	var payload struct {
		Metadata map[string]interface{} `json:"metadata"`
		Path     string                 `json:"path"`
		Model    string                 `json:"model"`
	}
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		log.Printf("Error unmarshalling message (%s) as json: %v", msg.LoggableID, err)

		// Acknowledge the message to prevent re-delivery since it is not processable.
		msg.Ack()
		return nil
	}

	if payload.Model == "" {
		log.Printf("Empty model in message: %s", msg.LoggableID)

		// Acknowledge the message to prevent re-delivery since it is not processable.
		msg.Ack()
		return nil
	}

	backendDeployment, backendExists := s.Deployments.ResolveDeployment(payload.Model)
	if !backendExists {
		log.Printf("Message (%s): deployment not found for model: %s", msg.LoggableID, payload.Model)

		// Hopefully the backend will be deployed soon or another subscriber will handle it.
		// Hopefully exponential backoff will be used to prevent overwhelming the backend.
		if msg.Nackable() {
			msg.Nack()
		}

		return nil
	}

	// Ensure the backend is scaled to at least one Pod.
	s.Deployments.AtLeastOne(backendDeployment)

	log.Printf("Entering queue: %s", msg.LoggableID)

	complete := s.Queues.EnqueueAndWait(ctx, backendDeployment, msg.LoggableID)
	defer complete()

	log.Printf("Awaiting host for message %s", msg.LoggableID)

	host, err := s.Endpoints.AwaitHostAddress(ctx, backendDeployment, "http")
	if err != nil {
		log.Printf("Error waiting for host for message %s: %v", msg.LoggableID, err)

		if msg.Nackable() {
			msg.Nack()
		}
		return nil
	}

	path := payload.Path
	if payload.Path == "" {
		// Default to completions endpoint.
		path = "/v1/completions"
	} else if !strings.HasPrefix(payload.Path, "/") {
		path = "/" + payload.Path
	}

	// TODO: Concurrency.

	url := fmt.Sprintf("http://%s%s", host, path)
	log.Printf("Sending request to backend for message %s: %s", msg.LoggableID, url)
	respPayload, err := s.sendRequest(ctx, url, msg.Body)
	if err != nil {
		log.Printf("Error sending request for message %s: %v", msg.LoggableID, err)

		if msg.Nackable() {
			msg.Nack()
		}
		return nil
	}

	var decodedRespPayload map[string]interface{}
	if err := json.Unmarshal(respPayload, &decodedRespPayload); err != nil {
		log.Printf("Error unmarshalling response for message %s: %v", msg.LoggableID, err)

		if msg.Nackable() {
			msg.Nack()
		}
		return nil
	}
	decodedRespPayload["metadata"] = payload.Metadata
	respPayloadWithMetadata, err := json.Marshal(decodedRespPayload)
	if err != nil {
		log.Printf("Error marshalling response with metadata for message %s: %v", msg.LoggableID, err)

		// Retrying wont fix, discard.
		msg.Ack()
		return nil
	}

	log.Printf("Sending response for message %s", msg.LoggableID)

	if err := s.responses.Send(ctx, &pubsub.Message{
		Body: respPayloadWithMetadata,
		Metadata: map[string]string{
			"request_message_id": msg.LoggableID,
		},
	}); err != nil {
		log.Printf("Error sending response for message %s: %v", msg.LoggableID, err)

		if msg.Nackable() {
			msg.Nack()
		}
		return nil
	}

	log.Printf("Successfully processed request, ack'ing message %s", msg.LoggableID)

	msg.Ack()
	return nil
}

func (s *Subscriber) sendRequest(ctx context.Context, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPC.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (s *Subscriber) Stop(ctx context.Context) error {
	return s.requests.Shutdown(ctx)
}
