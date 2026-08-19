package channel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestApplyUpstreamBodyMetadataAddsReplayableBody(t *testing.T) {
	payload := []byte(`{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)
	body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	defer closer.Close()

	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", body)
	require.NoError(t, err)
	assert.Nil(t, req.GetBody)
	assert.Zero(t, req.ContentLength)

	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: size,
		UpstreamRequestGetBody:  getBody,
	})
	assert.EqualValues(t, len(payload), req.ContentLength)
	require.NotNil(t, req.GetBody)

	first, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, first)
	for range 2 {
		replay, err := req.GetBody()
		require.NoError(t, err)
		got, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		assert.Equal(t, payload, got)
	}
}

func TestApplyUpstreamBodyMetadataKeepsNativeReplay(t *testing.T) {
	tests := []struct {
		name string
		body func() io.Reader
	}{
		{name: "bytes reader", body: func() io.Reader { return bytes.NewReader([]byte("original")) }},
		{name: "bytes buffer", body: func() io.Reader { return bytes.NewBufferString("original") }},
		{name: "strings reader", body: func() io.Reader { return strings.NewReader("original") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://example.com", test.body())
			require.NoError(t, err)
			require.NotNil(t, req.GetBody)
			ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
				UpstreamRequestBodySize: 99,
				UpstreamRequestGetBody: func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("override")), nil
				},
			})
			replay, err := req.GetBody()
			require.NoError(t, err)
			got, err := io.ReadAll(replay)
			require.NoError(t, err)
			require.NoError(t, replay.Close())
			assert.Equal(t, "original", string(got))
			assert.EqualValues(t, len("original"), req.ContentLength)
		})
	}
}

func TestUpstreamBodyMetadataResetsBetweenChannelAttempts(t *testing.T) {
	_, size, getBody, closer, err := relaycommon.NewOutboundJSONBody([]byte(`{"attempt":1}`))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{
		UpstreamRequestBodySize: size,
		UpstreamRequestGetBody:  getBody,
	}
	require.NoError(t, closer.Close())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info.InitChannelMeta(ctx)
	assert.Zero(t, info.UpstreamRequestBodySize)
	assert.Nil(t, info.UpstreamRequestGetBody)
}

func TestKeepUpstreamRedirectResponse(t *testing.T) {
	assert.ErrorIs(t, keepUpstreamRedirectResponse(nil, nil), http.ErrUseLastResponse)
}

type h2ReplayServerResult struct {
	err           error
	streamCount   int
	attemptBodies [][]byte
}

func runRefusedStreamServer(listener net.Listener, expectRetry bool) <-chan h2ReplayServerResult {
	results := make(chan h2ReplayServerResult, 1)
	go func() {
		result := h2ReplayServerResult{}
		defer func() { results <- result }()

		conn, err := listener.Accept()
		if err != nil {
			result.err = err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

		preface := make([]byte, len(http2.ClientPreface))
		if _, err := io.ReadFull(conn, preface); err != nil {
			result.err = fmt.Errorf("read client preface: %w", err)
			return
		}
		if !bytes.Equal(preface, []byte(http2.ClientPreface)) {
			result.err = fmt.Errorf("unexpected client preface")
			return
		}

		framer := http2.NewFramer(conn, conn)
		framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
		if err := framer.WriteSettings(); err != nil {
			result.err = err
			return
		}

		for attempt := 0; ; attempt++ {
			streamID, body, err := readH2Request(framer)
			if err != nil {
				result.err = err
				return
			}
			result.streamCount++
			result.attemptBodies = append(result.attemptBodies, body)
			if attempt == 0 {
				if err := framer.WriteRSTStream(streamID, http2.ErrCodeRefusedStream); err != nil {
					result.err = err
				}
				if !expectRetry {
					return
				}
				continue
			}

			var header bytes.Buffer
			encoder := hpack.NewEncoder(&header)
			if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
				result.err = err
				return
			}
			if err := framer.WriteHeaders(http2.HeadersFrameParam{
				StreamID: streamID, BlockFragment: header.Bytes(), EndHeaders: true,
			}); err != nil {
				result.err = err
				return
			}
			result.err = framer.WriteData(streamID, true, []byte(`{}`))
			return
		}
	}()
	return results
}

func readH2Request(framer *http2.Framer) (uint32, []byte, error) {
	var streamID uint32
	var body []byte
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return 0, nil, fmt.Errorf("read frame: %w", err)
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() {
				if err := framer.WriteSettingsAck(); err != nil {
					return 0, nil, err
				}
			}
		case *http2.MetaHeadersFrame:
			streamID = frame.Header().StreamID
			if frame.StreamEnded() {
				return streamID, body, nil
			}
		case *http2.DataFrame:
			if streamID == 0 {
				streamID = frame.Header().StreamID
			}
			if frame.Header().StreamID != streamID {
				continue
			}
			body = append(body, frame.Data()...)
			if frame.StreamEnded() {
				return streamID, body, nil
			}
		}
	}
}

func newH2ReplayClient(listener net.Listener) (*http.Client, *http2.Transport) {
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.Config) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, listener.Addr().String())
		},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}, transport
}

func waitForH2ReplayResult(t *testing.T, results <-chan h2ReplayServerResult) h2ReplayServerResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for HTTP/2 test server")
		return h2ReplayServerResult{}
	}
}

func TestUpstreamBodyHTTP2RetryAfterRefusedStream(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"retry me"}`)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	results := runRefusedStreamServer(listener, true)
	client, transport := newH2ReplayClient(listener)
	defer transport.CloseIdleConnections()

	body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	defer closer.Close()
	req, err := http.NewRequest(http.MethodPost, "http://upstream.test/v1/chat/completions", body)
	require.NoError(t, err)
	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: size, UpstreamRequestGetBody: getBody,
	})

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := waitForH2ReplayResult(t, results)
	require.NoError(t, result.err)
	assert.Equal(t, 2, result.streamCount)
	require.Len(t, result.attemptBodies, 2)
	assert.Equal(t, payload, result.attemptBodies[0])
	assert.Equal(t, payload, result.attemptBodies[1])
}

func TestUpstreamBodyHTTP2CannotRetryWithoutGetBody(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"retry me"}`)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	results := runRefusedStreamServer(listener, false)
	client, transport := newH2ReplayClient(listener)
	defer transport.CloseIdleConnections()

	body, size, _, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	defer closer.Close()
	req, err := http.NewRequest(http.MethodPost, "http://upstream.test/v1/chat/completions", body)
	require.NoError(t, err)
	applyUpstreamContentLength(req, &relaycommon.RelayInfo{UpstreamRequestBodySize: size})
	assert.Nil(t, req.GetBody)

	resp, err := client.Do(req)
	require.Error(t, err)
	assert.Nil(t, resp)
	require.ErrorContains(t, err, "cannot retry err")
	require.ErrorContains(t, err, "Request.Body was written")

	result := waitForH2ReplayResult(t, results)
	require.NoError(t, result.err)
	assert.Equal(t, 1, result.streamCount)
	require.Len(t, result.attemptBodies, 1)
	assert.Equal(t, payload, result.attemptBodies[0])
}
