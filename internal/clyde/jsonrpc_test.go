package clyde

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRPCCallReportsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	err := rpcCall(server.URL, "status.get", map[string]string{}, nil)

	if err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRPCLimitsRequestBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(strings.Repeat("x", maxJSONRPCBodyBytes+1)))
	rec := httptest.NewRecorder()

	handleRPC(rec, req, NewStatusStore())

	if !strings.Contains(rec.Body.String(), "invalid json") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}
