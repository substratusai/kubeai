package proxy

import (
	"bytes"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var _ http.Handler = &RetryMiddleware{}

type RetryMiddleware struct {
	other      http.Handler
	MaxRetries int
	rSource    *rand.Rand
}

func NewRetryMiddleware(maxRetries int, other http.Handler) *RetryMiddleware {
	if maxRetries < 1 {
		panic("invalid retries")
	}
	return &RetryMiddleware{
		other:      other,
		MaxRetries: maxRetries,
		rSource:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r RetryMiddleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var capturedResp *responseBuffer
	for i := 0; ; i++ {
		capturedResp = &responseBuffer{
			header: make(http.Header),
			body:   bytes.NewBuffer([]byte{}),
		}
		// call next handler in chain
		r.other.ServeHTTP(capturedResp, request.Clone(request.Context()))

		if i == r.MaxRetries || // max retries reached
			request.Context().Err() != nil || // abort early on timeout, context cancel
			capturedResp.status != http.StatusBadGateway &&
				capturedResp.status != http.StatusServiceUnavailable {
			break
		}
		totalRetries.Inc()
		// Exponential backoff
		jitter := time.Duration(r.rSource.Intn(50))
		time.Sleep(time.Millisecond*time.Duration(1<<uint(i)) + jitter)
	}
	for k, v := range capturedResp.header {
		writer.Header()[k] = v
	}
	writer.WriteHeader(capturedResp.status)
	if _, err := capturedResp.body.WriteTo(writer); err != nil {
		log.Printf("response write: %v", err)
	}
}

type responseBuffer struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (rb *responseBuffer) Header() http.Header {
	return rb.header
}

func (r *responseBuffer) WriteHeader(status int) {
	r.status = status
	r.header = r.Header().Clone()
}

func (rb *responseBuffer) Write(data []byte) (int, error) {
	return rb.body.Write(data)
}
