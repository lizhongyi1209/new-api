package relay

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	newapicommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type responsesUpstreamAttempt struct {
	mu sync.Mutex

	c     *gin.Context
	info  *relaycommon.RelayInfo
	audit *relaycommon.ResponsesTimingAudit

	startedAt         time.Time
	headersFinishedAt time.Time
	dnsStartedAt      time.Time
	dnsDoneAt         time.Time
	connectStartedAt  time.Time
	connectDoneAt     time.Time
	tlsStartedAt      time.Time
	tlsDoneAt         time.Time
	gotConnAt         time.Time
	wroteRequestAt    time.Time
	firstResponseAt   time.Time

	requestWrites int
	status        int
	protocol      string
	requestError  bool

	responseBody       *responsesTimingReadCloser
	downstreamWriter   *responsesTimingWriter
	responseStartedAt  time.Time
	responseFinishedAt time.Time
	finishOnce         sync.Once
}

type responsesTimingReadCloser struct {
	io.ReadCloser
	mu           sync.Mutex
	bytesRead    int64
	readDuration time.Duration
	lastReadAt   time.Time
}

func (r *responsesTimingReadCloser) Read(p []byte) (int, error) {
	startedAt := time.Now()
	n, err := r.ReadCloser.Read(p)
	finishedAt := time.Now()

	r.mu.Lock()
	r.bytesRead += int64(n)
	r.readDuration += finishedAt.Sub(startedAt)
	if n > 0 || err != nil {
		r.lastReadAt = finishedAt
	}
	r.mu.Unlock()
	return n, err
}

func (r *responsesTimingReadCloser) snapshot() (int64, time.Duration, time.Time) {
	if r == nil {
		return 0, 0, time.Time{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytesRead, r.readDuration, r.lastReadAt
}

type responsesTimingWriter struct {
	gin.ResponseWriter
	mu            sync.Mutex
	bytesWritten  int64
	writeDuration time.Duration
	firstWriteAt  time.Time
	lastWriteAt   time.Time
}

func (w *responsesTimingWriter) recordWrite(startedAt time.Time, bytes int) {
	finishedAt := time.Now()
	w.mu.Lock()
	if w.firstWriteAt.IsZero() {
		w.firstWriteAt = startedAt
	}
	w.lastWriteAt = finishedAt
	w.bytesWritten += int64(bytes)
	w.writeDuration += finishedAt.Sub(startedAt)
	w.mu.Unlock()
}

func (w *responsesTimingWriter) Write(data []byte) (int, error) {
	startedAt := time.Now()
	n, err := w.ResponseWriter.Write(data)
	w.recordWrite(startedAt, n)
	return n, err
}

func (w *responsesTimingWriter) WriteString(data string) (int, error) {
	startedAt := time.Now()
	n, err := w.ResponseWriter.WriteString(data)
	w.recordWrite(startedAt, n)
	return n, err
}

func (w *responsesTimingWriter) Flush() {
	startedAt := time.Now()
	w.ResponseWriter.Flush()
	w.recordWrite(startedAt, 0)
}

// Unwrap keeps http.ResponseController features such as write deadlines
// working while the writer is instrumented.
func (w *responsesTimingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responsesTimingWriter) snapshot() (int64, time.Duration, time.Duration) {
	if w == nil {
		return 0, 0, 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var total time.Duration
	if !w.firstWriteAt.IsZero() && !w.lastWriteAt.Before(w.firstWriteAt) {
		total = w.lastWriteAt.Sub(w.firstWriteAt)
	}
	return w.bytesWritten, w.writeDuration, total
}

func ensureResponsesTiming(c *gin.Context, info *relaycommon.RelayInfo) {
	if c == nil || info == nil ||
		info.RelayMode != relayconstant.RelayModeResponses ||
		c.Request == nil || c.Request.URL == nil ||
		c.Request.URL.Path != "/v1/responses" {
		return
	}
	if info.ResponsesTiming != nil {
		return
	}

	audit := &relaycommon.ResponsesTimingAudit{}
	if storage, err := newapicommon.GetBodyStorage(c); err == nil {
		audit.ClientRequestBytes = storage.Size()
	}
	requestReceivedAt := newapicommon.GetContextKeyTime(c, constant.ContextKeyRequestReceivedTime)
	requestBodyReadyAt := newapicommon.GetContextKeyTime(c, constant.ContextKeyRequestBodyReadyTime)
	if milliseconds := responsesDurationMilliseconds(requestReceivedAt, requestBodyReadyAt); milliseconds >= 0 {
		audit.ClientBodyReceiveMs = milliseconds
	}
	info.ResponsesTiming = audit
}

func newResponsesUpstreamAttempt(c *gin.Context, info *relaycommon.RelayInfo) (*responsesUpstreamAttempt, *httptrace.ClientTrace) {
	startedAt := time.Now()
	audit := info.ResponsesTiming
	if audit.UpstreamAttempts == 0 {
		requestBodyReadyAt := newapicommon.GetContextKeyTime(c, constant.ContextKeyRequestBodyReadyTime)
		if milliseconds := responsesDurationMilliseconds(requestBodyReadyAt, startedAt); milliseconds >= 0 {
			audit.LocalRequestMs = milliseconds
		}
	}
	audit.UpstreamAttempts++
	if info.UpstreamRequestBodySize > 0 {
		audit.UpstreamRequestBytes += info.UpstreamRequestBodySize
	}

	attempt := &responsesUpstreamAttempt{
		c:         c,
		info:      info,
		audit:     audit,
		startedAt: startedAt,
		protocol:  "unknown",
	}
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			attempt.mu.Lock()
			attempt.dnsStartedAt = time.Now()
			attempt.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			attempt.mu.Lock()
			attempt.dnsDoneAt = time.Now()
			attempt.mu.Unlock()
		},
		ConnectStart: func(_, _ string) {
			attempt.mu.Lock()
			attempt.connectStartedAt = time.Now()
			attempt.mu.Unlock()
		},
		ConnectDone: func(_, _ string, _ error) {
			attempt.mu.Lock()
			attempt.connectDoneAt = time.Now()
			attempt.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			attempt.mu.Lock()
			attempt.tlsStartedAt = time.Now()
			attempt.mu.Unlock()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			attempt.mu.Lock()
			attempt.tlsDoneAt = time.Now()
			attempt.mu.Unlock()
		},
		GotConn: func(httptrace.GotConnInfo) {
			attempt.mu.Lock()
			attempt.gotConnAt = time.Now()
			attempt.mu.Unlock()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			attempt.mu.Lock()
			attempt.wroteRequestAt = time.Now()
			attempt.requestWrites++
			attempt.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			attempt.mu.Lock()
			if attempt.firstResponseAt.IsZero() {
				attempt.firstResponseAt = time.Now()
			}
			attempt.mu.Unlock()
		},
	}
	return attempt, trace
}

func (a *responsesUpstreamAttempt) recordHeaders(response any, requestErr error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.headersFinishedAt = time.Now()
	a.requestError = requestErr != nil
	if response, ok := response.(*http.Response); ok && response != nil {
		a.status = response.StatusCode
		a.protocol = response.Proto
	}
}

func (a *responsesUpstreamAttempt) wrapResponseBody(response *http.Response) {
	if response == nil || response.Body == nil {
		return
	}
	a.responseBody = &responsesTimingReadCloser{ReadCloser: response.Body}
	response.Body = a.responseBody
}

func (a *responsesUpstreamAttempt) measureDownstream(c *gin.Context, callback func() (any, *types.NewAPIError)) (usage any, newAPIError *types.NewAPIError) {
	originalWriter := c.Writer
	a.downstreamWriter = &responsesTimingWriter{ResponseWriter: originalWriter}
	c.Writer = a.downstreamWriter
	a.responseStartedAt = time.Now()
	defer func() {
		a.responseFinishedAt = time.Now()
		c.Writer = originalWriter
	}()
	return callback()
}

func (a *responsesUpstreamAttempt) finish() {
	if a == nil {
		return
	}
	a.finishOnce.Do(func() {
		finishedAt := time.Now()
		responseBytes, responseReadDuration, lastReadAt := a.responseBody.snapshot()
		downstreamBytes, downstreamWriteDuration, downstreamTotalDuration := a.downstreamWriter.snapshot()

		a.mu.Lock()
		startedAt := a.startedAt
		headersFinishedAt := a.headersFinishedAt
		dnsStartedAt := a.dnsStartedAt
		dnsDoneAt := a.dnsDoneAt
		connectStartedAt := a.connectStartedAt
		connectDoneAt := a.connectDoneAt
		tlsStartedAt := a.tlsStartedAt
		tlsDoneAt := a.tlsDoneAt
		gotConnAt := a.gotConnAt
		wroteRequestAt := a.wroteRequestAt
		firstResponseAt := a.firstResponseAt
		requestWrites := a.requestWrites
		status := a.status
		protocol := a.protocol
		requestError := a.requestError
		a.mu.Unlock()

		upstreamFinishedAt := headersFinishedAt
		if lastReadAt.After(upstreamFinishedAt) {
			upstreamFinishedAt = lastReadAt
		}
		if upstreamFinishedAt.IsZero() {
			upstreamFinishedAt = finishedAt
		}

		upstreamTotalMilliseconds := responsesDurationMilliseconds(startedAt, upstreamFinishedAt)
		connectionMilliseconds := responsesDurationMilliseconds(startedAt, gotConnAt)
		dnsMilliseconds := responsesDurationMilliseconds(dnsStartedAt, dnsDoneAt)
		connectMilliseconds := responsesDurationMilliseconds(connectStartedAt, connectDoneAt)
		tlsMilliseconds := responsesDurationMilliseconds(tlsStartedAt, tlsDoneAt)
		requestWriteMilliseconds := responsesDurationMilliseconds(gotConnAt, wroteRequestAt)
		upstreamWaitMilliseconds := responsesDurationMilliseconds(wroteRequestAt, firstResponseAt)
		responseHeaderMilliseconds := responsesDurationMilliseconds(firstResponseAt, headersFinishedAt)
		firstEventMilliseconds := responsesDurationMilliseconds(headersFinishedAt, a.info.FirstResponseTime)

		accumulateResponsesDuration(&a.audit.UpstreamTotalMs, upstreamTotalMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamConnectionMs, connectionMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamDNSMs, dnsMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamConnectMs, connectMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamTLSMs, tlsMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamRequestWriteMs, requestWriteMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamWaitMs, upstreamWaitMilliseconds)
		accumulateResponsesDuration(&a.audit.UpstreamResponseHeaderMs, responseHeaderMilliseconds)
		if firstEventMilliseconds >= 0 {
			a.audit.UpstreamFirstEventMs += firstEventMilliseconds
		}
		a.audit.UpstreamResponseBytes += responseBytes
		a.audit.UpstreamResponseReadMs += responseReadDuration.Seconds() * 1000
		a.audit.UpstreamTransportAttempts += requestWrites
		if status > 0 {
			a.audit.UpstreamStatus = status
		}
		a.audit.DownstreamResponseBytes += downstreamBytes
		a.audit.DownstreamWriteMs += downstreamWriteDuration.Seconds() * 1000
		a.audit.DownstreamTotalMs += downstreamTotalDuration.Seconds() * 1000

		if !a.info.IsStream && !a.responseStartedAt.IsZero() && !a.responseFinishedAt.Before(a.responseStartedAt) {
			localResponseDuration := a.responseFinishedAt.Sub(a.responseStartedAt) - responseReadDuration - downstreamWriteDuration
			if localResponseDuration > 0 {
				a.audit.LocalResponseMs += localResponseDuration.Seconds() * 1000
			}
		}

		requestMegabitsPerSecond := -1.0
		if a.info.UpstreamRequestBodySize > 0 && requestWriteMilliseconds > 0 {
			requestMegabitsPerSecond = float64(a.info.UpstreamRequestBodySize) * 8 / 1000 / requestWriteMilliseconds
		}
		logger.LogInfo(a.c, fmt.Sprintf(
			"responses_timing: request_id=%s channel=%d attempt=%d transport_attempts=%d stream=%t status=%d protocol=%s request_bytes=%d total_ms=%.3f connection_ms=%.3f dns_ms=%.3f connect_ms=%.3f tls_ms=%.3f request_write_ms=%.3f request_mbps=%.3f upstream_wait_ms=%.3f response_header_ms=%.3f first_event_ms=%.3f response_bytes=%d response_read_ms=%.3f downstream_bytes=%d downstream_write_ms=%.3f downstream_total_ms=%.3f request_error=%t",
			a.info.RequestId,
			a.info.ChannelId,
			a.audit.UpstreamAttempts,
			requestWrites,
			a.info.IsStream,
			status,
			protocol,
			a.info.UpstreamRequestBodySize,
			upstreamTotalMilliseconds,
			connectionMilliseconds,
			dnsMilliseconds,
			connectMilliseconds,
			tlsMilliseconds,
			requestWriteMilliseconds,
			requestMegabitsPerSecond,
			upstreamWaitMilliseconds,
			responseHeaderMilliseconds,
			firstEventMilliseconds,
			responseBytes,
			responseReadDuration.Seconds()*1000,
			downstreamBytes,
			downstreamWriteDuration.Seconds()*1000,
			downstreamTotalDuration.Seconds()*1000,
			requestError,
		))
	})
}

func responsesDurationMilliseconds(start, end time.Time) float64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return -1
	}
	return end.Sub(start).Seconds() * 1000
}

func accumulateResponsesDuration(total *float64, milliseconds float64) {
	if milliseconds >= 0 {
		*total += milliseconds
	}
}
