package clyde

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

func ServeStatus(host string, port int, out io.Writer) error {
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return errf("Clyde daemon only binds to localhost addresses")
	}
	store := NewStatusStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/status") {
			writeJSON(w, http.StatusOK, store.Get(r.URL.Query().Get("job_id")))
			return
		}
		if r.Method == http.MethodPost && (r.URL.Path == "/" || r.URL.Path == "/rpc") {
			handleRPC(w, r, store)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	addr := net.JoinHostPort(host, itoa(int64(port)))
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Fprintf(out, "clyde daemon listening on http://%s/rpc\n", addr)
	return server.ListenAndServe()
}

func handleRPC(w http.ResponseWriter, r *http.Request, store *StatusStore) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRPCBodyBytes)
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, rpcError(nil, -32700, "invalid json: "+err.Error()))
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSON(w, http.StatusOK, rpcError(req.ID, -32600, "expected jsonrpc 2.0"))
		return
	}
	switch req.Method {
	case "daemon.ping":
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]bool{"ok": true}))
	case "status.get":
		var params struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(req.Params, &params)
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Get(params.JobID)))
	case "status.reset":
		var params struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(req.Params, &params)
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Reset(params.JobID)))
	case "status.event":
		var event ProgressEvent
		if err := json.Unmarshal(req.Params, &event); err != nil {
			writeJSON(w, http.StatusOK, rpcError(req.ID, -32602, "invalid status event: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Event(event)))
	default:
		writeJSON(w, http.StatusOK, rpcError(req.ID, -32601, "unknown method: "+req.Method))
	}
}
