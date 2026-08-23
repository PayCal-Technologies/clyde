package clyde

import "io"

type registeredCommand struct {
	Info commandInfo
	Run  func(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

func registeredCommands() []registeredCommand {
	return []registeredCommand{
		{Info: commandInfo{Name: "about", Category: "Core", Summary: "Show Clyde product details and official links.", Access: "Read-only", Network: "None", Syntax: "clyde --about\nclyde about", Examples: []string{"clyde --about", "clyde about"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			printAbout(stdout)
			return nil
		}},
		{Info: commandInfo{Name: "help", Category: "Core", Summary: "Show human-readable help or a JSON command catalog.", Access: "Read-only", Network: "None", Syntax: "clyde help [--json|COMMAND]", Examples: []string{"clyde help", "clyde help agent", "clyde help --json"}}, Run: cmdHelp},
		{Info: commandInfo{Name: "completion", Category: "Packaging", Summary: "Generate shell completion for Bash, Zsh, Fish, PowerShell, Elvish, Nushell, Xonsh, Tcsh, Clink, Yash, or Oil.", Access: "Read-only", Network: "None", Syntax: "clyde completion {bash|zsh|fish|powershell|pwsh|elvish|nushell|nu|xonsh|tcsh|clink|yash|oil|osh|ysh}", Examples: []string{"clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde", "clyde completion powershell | Out-String | Invoke-Expression", "clyde completion nushell > clyde-completions.nu"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdCompletion(args, stdout)
		}},
		{Info: commandInfo{Name: "doctor", Category: "Diagnostics", Summary: "Check Clyde configuration, platform, PATH tools, Ollama, and optional repo scan readiness.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde doctor [REPO] [--json] [--ollama-timeout SECONDS]", Examples: []string{"clyde doctor", "clyde doctor . --json"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdDoctor(args, stdout)
		}},
		{Info: commandInfo{Name: "tui", Category: "Interactive", Summary: "Open Clyde's dependency-free terminal UI.", Access: "Local interactive", Network: "Optional local Ollama", Syntax: "clyde tui", Examples: []string{"clyde", "clyde tui"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return RunTUI(stdin, stdout, stderr)
		}},
		{Info: commandInfo{Name: "config", Category: "Configuration", Summary: "Show, initialize, or print the Clyde configuration path.", Access: "Reads or writes local config", Network: "None", Syntax: "clyde config {show|init|path}", Examples: []string{"clyde config show", "clyde config init", "clyde config path"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdConfig(args, stdout)
		}},
		{Info: commandInfo{Name: "preview", Category: "Repository Bundles", Summary: "Report included and skipped repository files before bundling or upload.", Access: "Read-only", Network: "None", Syntax: "clyde preview REPO [scan flags] [--json]", Examples: []string{"clyde preview .", "clyde preview . --include 'internal/**/*.go' --json"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdPreview(args, stdout)
		}},
		{Info: commandInfo{Name: "scan-report", Category: "Repository Bundles", Summary: "Summarize repository scan shape with largest files, extensions, skip reasons, and chunk counts.", Access: "Read-only", Network: "None", Syntax: "clyde scan-report REPO [scan flags] [--json] [--top N]", Examples: []string{"clyde scan-report .", "clyde scan-report . --json", "clyde scan-report . --top 20"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdScanReport(args, stdout)
		}},
		{Info: commandInfo{Name: "bundle", Category: "Repository Bundles", Summary: "Write a digest-bound local manifest and source chunks for review. Secret-scan commands run against Clyde's captured source snapshot.", Access: "Writes local files", Network: "Optional local subprocess", Syntax: "clyde bundle REPO --out DIR [scan flags] [--force] [--secret-scan-command CMD] [--require-secret-scan]", Examples: []string{"clyde bundle . --out .clyde/out", "clyde bundle . --out .clyde/out --require-secret-scan --secret-scan-command \"gitleaks detect --no-git --source {repo}\""}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdBundle(args, stdout)
		}},
		{Info: commandInfo{Name: "sync", Category: "NotebookLM Sync", Summary: "Upload a verified bundle, or less-auditable live scan chunks, to NotebookLM after explicit approval.", Access: "Uploads repository chunks", Network: "NotebookLM backend", Syntax: "clyde sync --bundle DIR --approve-digest sha256:... --notebook-id ID --approve-upload [--receipt PATH] [--resume]\nclyde sync REPO --notebook-id ID --approve-upload [--receipt PATH]", Examples: []string{"clyde sync --bundle .clyde/out --approve-digest sha256:... --notebook-id nb --approve-upload", "clyde sync --bundle .clyde/out --approve-digest sha256:... --notebook-id nb --approve-upload --resume", "clyde sync . --notebook-id nb --approve-upload --receipt .clyde/live-sync-receipt.json"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdSync(args, stdout, stderr)
		}},
		{Info: commandInfo{Name: "daemon", Category: "Status", Summary: "Start the localhost-only JSON-RPC status daemon.", Access: "Starts local server", Network: "Localhost only", Syntax: "clyde daemon [--host HOST] [--port PORT]", Examples: []string{"clyde daemon"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdDaemon(args, stdout)
		}},
		{Info: commandInfo{Name: "status", Category: "Status", Summary: "Read Clyde daemon progress for a job.", Access: "Read-only", Network: "Localhost by default", Syntax: "clyde status [--job-id ID] [--json] [--watch]", Examples: []string{"clyde status", "clyde status --job-id clyde-sync"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdStatus(args, stdout)
		}},
		{Info: commandInfo{Name: "book", Category: "NotebookLM Sync", Summary: "Generate a dated NotebookLM book title and slug.", Access: "Read-only", Network: "None", Syntax: "clyde book SUBJECT...", Examples: []string{"clyde book Clyde self feedback"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdBook(args, stdout)
		}},
		{Info: commandInfo{Name: "models", Category: "Local Models", Summary: "List local Ollama models and the selected default.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde models [--json]", Examples: []string{"clyde models", "clyde models --json"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdModels(args, stdout)
		}},
		{Info: commandInfo{Name: "ask", Category: "Local Models", Summary: "Send a direct prompt to the selected local Ollama model.", Access: "Sends prompt to Ollama", Network: "Configured Ollama", Syntax: "clyde ask [flags] PROMPT", Examples: []string{"clyde ask 'Review this function'", "cat prompt.md | clyde ask --stdin"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdAsk(args, stdin, stdout)
		}},
		{Info: commandInfo{Name: "agent", Category: "Local Models", Summary: "Scan repository context and ask a local model for feedback.", Access: "Reads repo and sends selected context", Network: "Local Ollama by default", Syntax: "clyde agent REPO [scan flags] [PROMPT]", Examples: []string{"clyde agent . 'Review this repo'", "clyde agent . --include 'internal/**/*.go' --prompt-file review.md"}}, Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			return cmdAgent(args, stdin, stdout)
		}},
	}
}

func commandHandler(name string) (func(args []string, stdin io.Reader, stdout, stderr io.Writer) error, bool) {
	for _, command := range registeredCommands() {
		if command.Info.Name == name {
			return command.Run, true
		}
	}
	return nil, false
}

func registeredCommandNames() []string {
	commands := registeredCommands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Info.Name)
	}
	return names
}

func topLevelCommandNames() []string {
	return registeredCommandNames()
}
