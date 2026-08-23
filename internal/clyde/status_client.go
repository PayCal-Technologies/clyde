package clyde

import "net"

func StatusURL(host string, port int) string {
	return "http://" + net.JoinHostPort(host, itoa(int64(port))) + "/rpc"
}

func FetchStatus(url, jobID string) (map[string]any, error) {
	params := map[string]string{}
	if jobID != "" {
		params["job_id"] = jobID
	}
	var result map[string]any
	if err := rpcCall(url, "status.get", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
