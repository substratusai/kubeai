package proxy

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"
)

var _ http.Handler = &RetryMiddleware{}

type RetryMiddleware struct {
	nextHandler      http.Handler
	maxRetries       int
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
	}
}

// ServeHTTP handles the HTTP request by capturing the request body, calling the next handler in the chain, and retrying if necessary.
// It captures the request body using a LazyBodyCapturer, and sets a captured response writer using NewDiscardableResponseWriter.
// It retries the request if the response was discarded and the response status code is retryable.
// It uses exponential backoff for retries with a random jitter.
// The maximum number of retries is determined by the maxRetries field of RetryMiddleware.
func (r RetryMiddleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	lazyBody := NewLazyBodyCapturer(request.Body)
	request.Body = lazyBody
	for i := 0; ; i++ {
		withoutRetry := i == r.maxRetries || request.Context().Err() != nil
		if withoutRetry {
			r.nextHandler.ServeHTTP(writer, request)
			return
		}
		capturedResp := NewDiscardableResponseWriter(writer, r.isRetryableStatusCode)
		// call next handler in chain
		r.nextHandler.ServeHTTP(capturedResp, request.Clone(request.Context())) // clone also copies the reference to the lazy body capturer

		if !r.isRetryableStatusCode(capturedResp.CapturedStatusCode()) {
			break
		}
		// setup for retry
		lazyBody.Capture()

		totalRetries.Inc()
		// exponential backoff
		jitter := time.Duration(rand.Intn(50))
		time.Sleep(time.Millisecond*time.Duration(1<<uint(i)) + jitter)
	}
}

func (r RetryMiddleware) isRetryableStatusCode(status int) bool {
	// any 1xx informational response should be written
	if status >= 100 && status < 200 {
		return false
	}
	_, ok := r.retryStatusCodes[status]
	return ok
}

type (
	StatusCodeCapturer interface {
		CapturedStatusCode() int
	}
	XResponseWriter interface {
		http.ResponseWriter
		StatusCodeCapturer
	}

	// CapturedBodySource represents an interface for capturing and retrieving the body of an HTTP request.
	CapturedBodySource interface {
		IsCaptured() bool
		GetBody() []byte
	}

	XBodyCapturer interface {
		io.ReadCloser
		CapturedBodySource
		Capture()
	}
)

var (
	_ http.Flusher                    = &discardableResponseWriter{}
	_ io.ReaderFrom                   = &discardableResponseWriterWithReaderFrom{}
	_ CaptureStatusCodeResponseWriter = &discardableResponseWriter{}
)

// discardableResponseWriter represents a wrapper around http.ResponseWriter that provides additional
// functionalities for handling response writing. Depending on the status code,
// the header + content on write is skipped so that it can be re-used on retry or written to the underlying
// response writer.
type discardableResponseWriter struct {
	http.ResponseWriter
	headerBuf      http.Header
	wroteHeader    bool
	statusCode     int
	immediateWrite bool
	isDiscardable  func(status int) bool
}

// NewDiscardableResponseWriter creates a new instance of the response writer delegator.
// It takes a http.ResponseWriter and a function to determine by the status code if content should be written or discarded (for retry).
func NewDiscardableResponseWriter(writer http.ResponseWriter, isDiscardable func(status int) bool) XResponseWriter {
	d := &discardableResponseWriter{
		isDiscardable:  isDiscardable,
		ResponseWriter: writer,
		headerBuf:      make(http.Header),
		immediateWrite: false,
	}
	if _, ok := writer.(io.ReaderFrom); ok {
		return &discardableResponseWriterWithReaderFrom{discardableResponseWriter: d}
	}
	return d
}

func (r *discardableResponseWriter) CapturedStatusCode() int {
	return r.statusCode
}

func (r *discardableResponseWriter) Header() http.Header {
	return r.headerBuf
}

// WriteHeader sets the response status code and writes the response header to the underlying http.ResponseWriter or
// discards it based on the result of the isDiscardable call.
func (r *discardableResponseWriter) WriteHeader(status int) {
	r.statusCode = status
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	if r.isDiscardable(status) {
		return
	}
	r.immediateWrite = true
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
// Based on the status code, the content is either written or discarded.
//
// It returns the number of bytes written and any error encountered.
func (r *discardableResponseWriter) Write(data []byte) (int, error) {
	// ensure header is set. default is 200 in Go stdlib
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.immediateWrite {
		return r.ResponseWriter.Write(data)
	}
	return io.Discard.Write(data)
}

func (r *discardableResponseWriter) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// discardableResponseWriterWithReaderFrom provides the same functionalities as discardableResponseWriter but also implements the
// io.ReaderFrom interface.
// Based on the status code, the content is either written or discarded.
type discardableResponseWriterWithReaderFrom struct {
	*discardableResponseWriter
}

func (r *discardableResponseWriterWithReaderFrom) ReadFrom(re io.Reader) (int64, error) {
	// ensure header is set. default is 200 in Go stdlib
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.immediateWrite {
		return r.ResponseWriter.(io.ReaderFrom).ReadFrom(re)
	}
	return io.Copy(io.Discard, re)
}

var (
	_ io.ReadCloser = &lazyBodyCapturer{}
	_ io.WriterTo   = &lazyBodyCapturerWriteTo{}
)

// lazyBodyCapturer represents a type that captures the request body lazily.
// It wraps an io.ReadCloser and provides methods for reading, closing,
// writing to an io.Writer, and capturing the body content.
type lazyBodyCapturer struct {
	reader       io.ReadCloser
	capturedBody []byte
	buf          *bytes.Buffer
	allRead      bool
}

// NewLazyBodyCapturer constructor.
func NewLazyBodyCapturer(body io.ReadCloser) XBodyCapturer {
	c := &lazyBodyCapturer{
		reader: body,
		buf:    bytes.NewBuffer([]byte{}),
	}
	if _, ok := c.reader.(io.WriterTo); ok {
		return &lazyBodyCapturerWriteTo{c}
	}
	return c
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

// Capture marks the body as fully captured.
// The captured body data can be read via GetBody.
func (m *lazyBodyCapturer) Capture() {
	m.allRead = true
	if !m.IsCaptured() {
		m.capturedBody = m.buf.Bytes()
		m.buf = nil
	}
	m.reader = io.NopCloser(bytes.NewReader(m.capturedBody))
}

// IsCaptured returns true when a body was captured.
func (m *lazyBodyCapturer) IsCaptured() bool {
	return m.capturedBody != nil
}

// GetBody returns the captured byte slice. Value is nil when not captured, yet.
func (m *lazyBodyCapturer) GetBody() []byte {
	return m.capturedBody
}

type lazyBodyCapturerWriteTo struct {
	*lazyBodyCapturer
}

func (m *lazyBodyCapturerWriteTo) WriteTo(w io.Writer) (int64, error) {
	if m.allRead {
		return m.reader.(io.WriterTo).WriteTo(w)
	}
	n, err := m.reader.(io.WriterTo).WriteTo(io.MultiWriter(w, m.buf))
	m.allRead = true
	return n, err
}
