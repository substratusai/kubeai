package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeHTTP(t *testing.T) {
	myHeader := map[string][]string{"Foo": {"bar1", "bar2"}}
	specs := map[string]struct {
		context    func() context.Context
		maxRetries int
		headers    http.Header
		respStatus int
		expRetries int
	}{
		"no retry on 200": {
			context:    func() context.Context { return context.TODO() },
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusOK,
			expRetries: 0,
		},
		"no retry on 500": {
			context:    func() context.Context { return context.TODO() },
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusInternalServerError,
			expRetries: 0,
		},
		"max retries on 503": {
			context:    func() context.Context { return context.TODO() },
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusServiceUnavailable,
			expRetries: 3,
		},
		"max retries on 502": {
			context:    func() context.Context { return context.TODO() },
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusBadGateway,
			expRetries: 3,
		},
		"not buffered on 100": {
			context:    func() context.Context { return context.TODO() },
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusContinue,
			expRetries: 0,
		},
		"context cancelled": {
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.TODO())
				cancel()
				return ctx
			},
			headers:    myHeader,
			maxRetries: 3,
			respStatus: http.StatusBadGateway,
			expRetries: 0,
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			counterBefore := counterValue(t, totalRetries)
			const myBody = "my-request-body"
			req, err := http.NewRequestWithContext(spec.context(), "GET", "/test", strings.NewReader(myBody))
			require.NoError(t, err)
			req.Header = spec.headers.Clone()

			respRecorder := httptest.NewRecorder()

			var counter int
			testBackend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				// return all headers
				for k, vals := range r.Header {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(spec.respStatus)
				reqBody, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				_, err = w.Write(append(reqBody, []byte(strconv.Itoa(spec.respStatus))...))
				require.NoError(t, err)
			})

			// when
			middleware := NewRetryMiddleware(spec.maxRetries, testBackend)
			middleware.ServeHTTP(respRecorder, req)

			// then
			resp := respRecorder.Result()
			require.Equal(t, spec.respStatus, resp.StatusCode)
			assert.Equal(t, spec.expRetries, counter-1)
			// and headers matches
			assert.Equal(t, spec.headers, resp.Header)
			// and body matches
			bodyRead, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, myBody+strconv.Itoa(spec.respStatus), string(bodyRead))
			// and prometheus metric updated
			assert.Equal(t, spec.expRetries, int(counterValue(t, totalRetries)-counterBefore))
		})
	}
}

func TestWriteDelegatorReadFrom(t *testing.T) {
	const myTestContent = `my body content`

	rec := &testResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	// scenario: discard on error disabled
	d := newResponseWriterDelegator(rec, func(int) bool { return true }, false)
	// when
	n, err := d.(io.ReaderFrom).ReadFrom(strings.NewReader(myTestContent))
	// then the content is written
	require.NoError(t, err)
	assert.Equal(t, len(myTestContent), int(n))
	assert.Equal(t, myTestContent, rec.Body.String())
	assert.Equal(t, http.StatusOK, d.capturedStatusCode())

	// scenario: discard on error enabled
	rec = &testResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	// with discard on error disabled
	d = newResponseWriterDelegator(rec, func(int) bool { return true }, true)
	// when
	n, err = d.(io.ReaderFrom).ReadFrom(strings.NewReader(myTestContent))
	// then the content is not written
	require.NoError(t, err)
	assert.Equal(t, len(myTestContent), int(n))
	assert.Equal(t, "", rec.Body.String())
	assert.Equal(t, http.StatusOK, d.capturedStatusCode())

	// scenario: not implementing io.ReaderFrom
	d = newResponseWriterDelegator(httptest.NewRecorder(), func(int) bool { return true }, false)
	_, ok := d.(io.ReaderFrom)
	require.False(t, ok)
}

func TestLazyBodyCapturer(t *testing.T) {
	const myTestContent = "my-test-content"
	c := newLazyBodyCapturer(io.NopCloser(strings.NewReader(myTestContent)))
	var buf bytes.Buffer
	n, err := c.(io.WriterTo).WriteTo(&buf)
	require.NoError(t, err)
	assert.Len(t, myTestContent, int(n))
	assert.Equal(t, myTestContent, buf.String())
	// when captured
	c.Capture()
	// then data is buffered for second read
	buf.Reset()
	n, err = c.(io.WriterTo).WriteTo(&buf)
	require.NoError(t, err)
	assert.Equal(t, len(myTestContent), 15)
	assert.Equal(t, myTestContent, buf.String())

	// scenario: source reader does not implement WriteTo
	c = newLazyBodyCapturer(testReader{strings.NewReader(myTestContent)})
	// then instance also does not implement it
	_, ok := c.(io.WriterTo)
	require.False(t, ok)
}

func counterValue(t *testing.T, counter prometheus.Counter) float64 {
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(counter)
	gathered, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, gathered, 1)
	require.Len(t, gathered[0].Metric, 1)
	return gathered[0].Metric[0].GetCounter().GetValue()
}

type testResponseWriter struct {
	*httptest.ResponseRecorder
}

func (r *testResponseWriter) ReadFrom(re io.Reader) (int64, error) {
	return r.ResponseRecorder.Body.ReadFrom(re)
}

type testReader struct {
	r io.Reader
}

func (t testReader) Read(p []byte) (n int, err error) {
	return t.r.Read(p)
}

func (t testReader) Close() error {
	return nil
}
