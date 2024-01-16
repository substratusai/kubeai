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
	nextHandler http.Handler
	MaxRetries  int
	rSource     *rand.Rand
}

func NewRetryMiddleware(maxRetries int, other http.Handler) *RetryMiddleware {
	if maxRetries < 1 {
		panic("invalid retries")
	}
	return &RetryMiddleware{
		nextHandler: other,
		MaxRetries:  maxRetries,
		rSource:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r RetryMiddleware) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	lazyBody := &lazyBodyCapturer{
		reader: request.Body,
		buf:    bytes.NewBuffer([]byte{}),
	}
	request.Body = lazyBody
	var capturedResp *responseWriterDelegator
	for i := 0; ; i++ {
		capturedResp = &responseWriterDelegator{
			ResponseWriter: writer,
			headerBuf:      make(http.Header),
			discardErrResp: i < r.MaxRetries &&
				request.Context().Err() == nil, // abort early on timeout, context cancel
		}
		// call next handler in chain
		req, err := http.NewRequestWithContext(request.Context(), request.Method, request.URL.String(), lazyBody)
		if err != nil {
			panic(err)
		}
		r.nextHandler.ServeHTTP(capturedResp, req)
		lazyBody.Capture()
		if !capturedResp.discardErrResp || // max retries reached
			!isRetryableStatusCode(capturedResp.statusCode) {
			break
		}
		totalRetries.Inc()
		// Exponential backoff
		jitter := time.Duration(r.rSource.Intn(50))
		time.Sleep(time.Millisecond*time.Duration(1<<uint(i)) + jitter)
	}
}

func isRetryableStatusCode(status int) bool {
	return status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

var (
	_ http.Flusher  = &responseWriterDelegator{}
	_ io.ReaderFrom = &responseWriterDelegator{}
)

type responseWriterDelegator struct {
	http.ResponseWriter
	headerBuf   http.Header
	wroteHeader bool
	statusCode  int
	// always writes to responseWriter when false
	discardErrResp bool
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
	if r.discardErrResp && isRetryableStatusCode(status) {
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

func (r *responseWriterDelegator) Write(data []byte) (int, error) {
	// ensure header is set. default is 200 in Go stdlib
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.discardErrResp && isRetryableStatusCode(r.statusCode) {
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
	if r.discardErrResp && isRetryableStatusCode(r.statusCode) {
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
