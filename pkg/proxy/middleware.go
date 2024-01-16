package proxy

import (
	"bytes"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var _ http.Handler = &RetryMiddleware{}

type RetryMiddleware struct {
	nextHandler      http.Handler
	maxRetries       int
	rSource          *rand.Rand
	retryStatusCodes map[int]struct{}
}

// NewRetryMiddleware creates a new HTTP middleware that adds retry functionality.
// It takes the maximum number of retries, the next handler in the middleware chain,
// and an optional list of retryable status codes.
// If the maximum number of retries is 0, it returns the next handler without adding any retries.
// If the list of retryable status codes is empty, it uses a default set of status codes (http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout).
// The function creates a RetryMiddleware struct with the given parameters and returns it as an http.Handler.
func NewRetryMiddleware(maxRetries int, other http.Handler, optRetryStatusCodes ...int) http.Handler {
	if maxRetries == 0 {
		return other
	}
	if len(optRetryStatusCodes) == 0 {
		optRetryStatusCodes = []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	}
	statusCodeIndex := make(map[int]struct{}, len(optRetryStatusCodes))
	for _, c := range optRetryStatusCodes {
		statusCodeIndex[c] = struct{}{}
	}
	return &RetryMiddleware{
		nextHandler:      other,
		maxRetries:       maxRetries,
		retryStatusCodes: statusCodeIndex,
		rSource:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r RetryMiddleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	lazyBody := &lazyBodyCapturer{
		reader: request.Body,
		buf:    bytes.NewBuffer([]byte{}),
	}
	request.Body = lazyBody
	for i := 0; ; i++ {
		capturedResp := &responseWriterDelegator{
			isRetryableStatusCode: r.isRetryableStatusCode,
			ResponseWriter:        writer,
			headerBuf:             make(http.Header),
			discardErrResp: i < r.maxRetries &&
				request.Context().Err() == nil, // abort early on timeout, context cancel
		}
		// call next handler in chain
		req, err := http.NewRequestWithContext(request.Context(), request.Method, request.URL.String(), lazyBody)
		if err != nil {
			log.Printf("clone request: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.nextHandler.ServeHTTP(capturedResp, req)
		lazyBody.Capture()
		if !capturedResp.discardErrResp || // max retries reached
			!r.isRetryableStatusCode(capturedResp.statusCode) {
			break
		}
		totalRetries.Inc()
		// Exponential backoff
		jitter := time.Duration(r.rSource.Intn(50))
		time.Sleep(time.Millisecond*time.Duration(1<<uint(i)) + jitter)
	}
}

func (r RetryMiddleware) isRetryableStatusCode(status int) bool {
	_, ok := r.retryStatusCodes[status]
	return ok
}

var (
	_ http.Flusher  = &responseWriterDelegator{}
	_ io.ReaderFrom = &responseWriterDelegator{}
)

// responseWriterDelegator represents a wrapper around http.ResponseWriter that provides additional
// functionalities for handling response writing. Depending on the status code and discard settings,
// the heeader + content on write is skipped so that it can be re-used on retry.
type responseWriterDelegator struct {
	http.ResponseWriter
	headerBuf   http.Header
	wroteHeader bool
	statusCode  int
	// always writes to responseWriter when false
	discardErrResp        bool
	isRetryableStatusCode func(status int) bool
}

func (r *responseWriterDelegator) Header() http.Header {
	return r.headerBuf
}

func (r *responseWriterDelegator) WriteHeader(status int) {
	r.statusCode = status
	if !r.wroteHeader {
		r.wroteHeader = true
		// any 1xx informational response should be written
		r.discardErrResp = r.discardErrResp && !(status >= 100 && status < 200)
	}
	if r.discardErrResp && r.isRetryableStatusCode(status) {
		return
	}
	// copy header values to target
	for k, vals := range r.headerBuf {
		for _, val := range vals {
			r.ResponseWriter.Header().Add(k, val)
		}
	}
	r.ResponseWriter.WriteHeader(status)
}

// Write writes data to the response.
// If the response header has not been set, it sets the default status code to 200.
// When the status code qualifies for a retry, no content is written.
//
// It returns the number of bytes written and any error encountered.
func (r *responseWriterDelegator) Write(data []byte) (int, error) {
	// ensure header is set. default is 200 in Go stdlib
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.discardErrResp && r.isRetryableStatusCode(r.statusCode) {
		return io.Discard.Write(data)
	} else {
		return r.ResponseWriter.Write(data)
	}
}

func (r *responseWriterDelegator) ReadFrom(re io.Reader) (int64, error) {
	// ensure header is set. default is 200 in Go stdlib
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.discardErrResp && r.isRetryableStatusCode(r.statusCode) {
		return io.Copy(io.Discard, re)
	} else {
		return r.ResponseWriter.(io.ReaderFrom).ReadFrom(re)
	}
}

func (r *responseWriterDelegator) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var (
	_ io.ReadCloser = &lazyBodyCapturer{}
	_ io.WriterTo   = &lazyBodyCapturer{}
)

type lazyBodyCapturer struct {
	reader       io.ReadCloser
	capturedBody []byte
	buf          *bytes.Buffer
	allRead      bool
}

func (m *lazyBodyCapturer) Read(p []byte) (int, error) {
	if m.allRead {
		return m.reader.Read(p)
	}
	n, err := io.TeeReader(m.reader, m.buf).Read(p)
	if errors.Is(err, io.EOF) {
		m.allRead = true
	}
	return n, err
}

func (m *lazyBodyCapturer) Close() error {
	return m.reader.Close()
}

func (m *lazyBodyCapturer) WriteTo(w io.Writer) (int64, error) {
	if m.allRead {
		return m.reader.(io.WriterTo).WriteTo(w)
	}
	n, err := m.reader.(io.WriterTo).WriteTo(io.MultiWriter(w, m.buf))
	if errors.Is(err, io.EOF) {
		m.allRead = true
	}
	return n, err
}

func (m *lazyBodyCapturer) Capture() {
	m.allRead = true
	if m.buf != nil {
		m.capturedBody = m.buf.Bytes()
		m.buf = nil
	} else {
		m.reader = io.NopCloser(bytes.NewReader(m.capturedBody))
	}
}
