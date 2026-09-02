package common

// ResponsesTimingAudit records aggregate durations and byte counts for
// /v1/responses. It intentionally excludes request bodies, response bodies,
// URLs, credentials, and other user or upstream content.
type ResponsesTimingAudit struct {
	ClientRequestBytes  int64   `json:"client_request_bytes,omitempty"`
	ClientBodyReceiveMs float64 `json:"client_body_receive_ms,omitempty"`
	LocalRequestMs      float64 `json:"local_request_ms,omitempty"`

	UpstreamRequestBytes      int64   `json:"upstream_request_bytes,omitempty"`
	UpstreamTotalMs           float64 `json:"upstream_total_ms,omitempty"`
	UpstreamConnectionMs      float64 `json:"upstream_connection_ms,omitempty"`
	UpstreamDNSMs             float64 `json:"upstream_dns_ms,omitempty"`
	UpstreamConnectMs         float64 `json:"upstream_connect_ms,omitempty"`
	UpstreamTLSMs             float64 `json:"upstream_tls_ms,omitempty"`
	UpstreamRequestWriteMs    float64 `json:"upstream_request_write_ms,omitempty"`
	UpstreamWaitMs            float64 `json:"upstream_wait_ms,omitempty"`
	UpstreamResponseHeaderMs  float64 `json:"upstream_response_header_ms,omitempty"`
	UpstreamFirstEventMs      float64 `json:"upstream_first_event_ms,omitempty"`
	UpstreamResponseBytes     int64   `json:"upstream_response_bytes,omitempty"`
	UpstreamResponseReadMs    float64 `json:"upstream_response_read_ms,omitempty"`
	UpstreamAttempts          int     `json:"upstream_attempts,omitempty"`
	UpstreamTransportAttempts int     `json:"upstream_transport_attempts,omitempty"`
	UpstreamStatus            int     `json:"upstream_status,omitempty"`

	LocalResponseMs         float64 `json:"local_response_ms,omitempty"`
	DownstreamResponseBytes int64   `json:"downstream_response_bytes,omitempty"`
	DownstreamWriteMs       float64 `json:"downstream_write_ms,omitempty"`
	DownstreamTotalMs       float64 `json:"downstream_total_ms,omitempty"`
}
