package clyde

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxJSONRPCBodyBytes = 1 << 20
	jsonRPCTimeout      = 10 * time.Second
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, _ := json.MarshalIndent(payload, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func rpcResult(id any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func rpcError(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func rpcCall(url, method string, params any, out any) error {
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), jsonRPCTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxJSONRPCBodyBytes))
		return errf("JSON-RPC call failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONRPCBodyBytes)).Decode(&decoded); err != nil {
		return err
	}
	if decoded.Error != nil {
		return errf("%s", decoded.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(decoded.Result, out)
	}
	return nil
}
