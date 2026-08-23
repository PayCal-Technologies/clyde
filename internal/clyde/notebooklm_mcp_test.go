package clyde

import "testing"

func TestDefaultMCPEnvDoesNotSetDevelopmentAccount(t *testing.T) {
	if _, ok := defaultMCPEnv["NOTEBOOKLM_ACCOUNT"]; ok {
		t.Fatal("default MCP environment must not force NOTEBOOKLM_ACCOUNT")
	}
}
