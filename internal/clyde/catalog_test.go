package clyde

import (
	"strings"
	"testing"
)

func TestCatalogHelpersExposeTopLevelCommandsAndShells(t *testing.T) {
	wantCommands := "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent"
	if got := topLevelCommandList(); got != wantCommands {
		t.Fatalf("topLevelCommandList()=%q", got)
	}
	if got := topLevelCommandUsageList(); got != strings.ReplaceAll(wantCommands, " ", ",") {
		t.Fatalf("topLevelCommandUsageList()=%q", got)
	}
	if got := strings.Join(registeredCommandNames(), " "); got != wantCommands {
		t.Fatalf("registeredCommandNames()=%q", got)
	}
	wantShells := "bash, zsh, fish, powershell, pwsh, elvish, nushell, nu, xonsh, tcsh, clink, yash, oil, osh, ysh"
	if got := supportedShellList(); got != wantShells {
		t.Fatalf("supportedShellList()=%q", got)
	}
	if got := supportedShellChoiceList(); got != strings.ReplaceAll(wantShells, ", ", "|") {
		t.Fatalf("supportedShellChoiceList()=%q", got)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "pwsh", "elvish", "nushell", "nu", "xonsh", "tcsh", "clink", "yash", "oil", "osh", "ysh"} {
		if !supportedShell(shell) {
			t.Fatalf("supportedShell(%q)=false", shell)
		}
	}
	if supportedShell("planet9") {
		t.Fatal("unsupported shell reported as supported")
	}
}

func TestCompletionScriptsContainCatalogCommandsAndShells(t *testing.T) {
	for _, target := range completionTargets() {
		name := strings.Join(target.Names, "/")
		for _, command := range topLevelCommandNames() {
			if !strings.Contains(target.Script, command) {
				t.Fatalf("%s completion missing command %q", name, command)
			}
		}
		for _, shell := range supportedCompletionShells() {
			if !strings.Contains(target.Script, shell) {
				t.Fatalf("%s completion missing shell %q", name, shell)
			}
		}
	}
}

func TestCompletionShellAliasesResolveToExpectedScripts(t *testing.T) {
	aliases := map[string]string{
		"pwsh": "Register-ArgumentCompleter",
		"nu":   `extern "clyde"`,
		"osh":  "Oil/OSH/YSH completion",
		"ysh":  "Oil/OSH/YSH completion",
	}
	for shell, want := range aliases {
		script, ok := completionScriptForShell(shell)
		if !ok {
			t.Fatalf("completionScriptForShell(%q) not found", shell)
		}
		if !strings.Contains(script, want) {
			t.Fatalf("completionScriptForShell(%q) missing %q", shell, want)
		}
	}
}
