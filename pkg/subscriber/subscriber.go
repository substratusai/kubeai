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
	"time"

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
	var consecutiveErrors int
	for {
		msg, err := s.requests.Receive(ctx)
		if err != nil {
			return err
		}

		log.Println("Received message:", msg.LoggableID)

		if err := s.handleRequest(context.Background(), msg); err != nil {
			log.Printf("Error handling message %s: %v", msg.LoggableID, err)
			consecutiveErrors++

			// Slow down a bit to avoid churning through messages and running
			// up cloud costs when no meaningful work is being done.
			wait := consecutiveErrBackoff(consecutiveErrors)
			log.Printf("after %d consecutive errors, waiting %v before processing next message", consecutiveErrors, wait)
			time.Sleep(wait)
		} else {
			consecutiveErrors = 0
		}
	}
}

func consecutiveErrBackoff(n int) time.Duration {
	d := time.Duration(n) * time.Second
	const maxBackoff = 3 * time.Minute
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func (s *Subscriber) handleRequest(ctx context.Context, msg *pubsub.Message) error {
	// Expecting a message with the following structure:
	/*
		{
			"metadata": {
				"some-sort-of-id": 123,
				"optional-key": "some-user-value"
				# ...
			},
			"path": "/v1/completions",
			"body": {
				"model": "test-model"
				# ... other OpenAI compatible fields
			}
		}
	*/
	req, err := parseRequest(ctx, msg)
	if err != nil {
		return s.sendResponse(req, jsonError("error parsing request: %v", err), http.StatusBadRequest)
	}

	backendDeployment, backendExists := s.Deployments.ResolveDeployment(req.model)
	if !backendExists {
		// Send a 400 response to the client, however it is possible the backend
		// will be deployed soon or another subscriber will handle it.
		return s.sendResponse(req, jsonError("backend not found for model: %v", req.model), http.StatusNotFound)
	}

	// Ensure the backend is scaled to at least one Pod.
	s.Deployments.AtLeastOne(backendDeployment)

	log.Printf("Entering queue: %s", msg.LoggableID)

	complete := s.Queues.EnqueueAndWait(ctx, backendDeployment, msg.LoggableID)
	// TODO: Make sure complete() is called at the right time once code is modified to launch a goroutine
	// to support concurrency.
	defer complete()

	log.Printf("Awaiting host for message %s", msg.LoggableID)

	host, err := s.Endpoints.AwaitHostAddress(ctx, backendDeployment, "http")
	if err != nil {
		return s.sendResponse(req, jsonError("error awaiting host for backend: %v", err), http.StatusBadGateway)
	}

	// TODO: Concurrency.

	url := fmt.Sprintf("http://%s%s", host, req.path)
	log.Printf("Sending request to backend for message %s: %s", msg.LoggableID, url)
	respPayload, respCode, err := s.sendBackendRequest(ctx, url, req.body)
	if err != nil {
		return s.sendResponse(req, jsonError("error sending request to backend: %v", err), http.StatusBadGateway)
	}

	return s.sendResponse(req, respPayload, respCode)
}

func (s *Subscriber) sendBackendRequest(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPC.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return payload, resp.StatusCode, nil
}

var (
	ErrFailedToSendResponse = fmt.Errorf("failed to send response")
)

func (s *Subscriber) sendResponse(req *request, body []byte, statusCode int) error {
	log.Printf("Sending response to message: %v", req.msg.LoggableID)

	response := struct {
		Metadata   map[string]interface{} `json:"metadata"`
		StatusCode int                    `json:"status_code"`
		Body       json.RawMessage        `json:"body"`
	}{
		Metadata:   req.metadata,
		StatusCode: statusCode,
		Body:       body,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		jsonResponse = []byte(fmt.Sprintf(`{"error": "error marshalling response: %v"}`, err))
	}

	if err := s.responses.Send(req.ctx, &pubsub.Message{
		Body: jsonResponse,
		Metadata: map[string]string{
			"request_message_id": req.msg.LoggableID,
		},
	}); err != nil {
		log.Printf("Error sending response for message %s: %v", req.msg.LoggableID, err)

		// If a repsonse cant be sent, the message should be redelivered.
		if req.msg.Nackable() {
			req.msg.Nack()
		}
		return fmt.Errorf("%w: %v", ErrFailedToSendResponse, err)
	}

	log.Printf("Send response for message: %s", req.msg.LoggableID)
	req.msg.Ack()
	return nil
}

func (s *Subscriber) Stop(ctx context.Context) error {
	return s.requests.Shutdown(ctx)
}

type request struct {
	ctx      context.Context
	msg      *pubsub.Message
	metadata map[string]interface{}
	path     string
	body     json.RawMessage
	model    string
}

func parseRequest(ctx context.Context, msg *pubsub.Message) (*request, error) {
	var payload struct {
		Metadata map[string]interface{} `json:"metadata"`
		Path     string                 `json:"path"`
		Body     json.RawMessage        `json:"body"`
	}
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshalling message (%s) as json: %w", msg.LoggableID, err)
	}

	var payloadBody struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(payload.Body, &payloadBody); err != nil {
		return nil, fmt.Errorf("unmarshalling message (%s) .body as json: %w", msg.LoggableID, err)
	}

	if payloadBody.Model == "" {
		return nil, fmt.Errorf("empty model in message: %s", msg.LoggableID)
	}

	path := payload.Path
	if payload.Path == "" {
		// Default to completions endpoint.
		path = "/v1/completions"
	} else if !strings.HasPrefix(payload.Path, "/") {
		path = "/" + payload.Path
	}

	return &request{
		ctx:      ctx,
		msg:      msg,
		metadata: payload.Metadata,
		path:     path,
		body:     payload.Body,
		model:    payloadBody.Model,
	}, nil
}

func jsonError(format string, args ...interface{}) []byte {
	s := fmt.Sprintf(format, args...)
	fmt.Println(s)
	// Example OpenAI error response:
	/*
		{
		  "error": {
		    "message": "Invalid authorization header",
		    "type": "server_error",
		    "param": null,
		    "code": null
		  }
	*/
	return []byte(fmt.Sprintf(`{
	"error": {
		"message": %q
	}
}`, s))
}
