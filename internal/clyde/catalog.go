package clyde

type commandInfo struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Access   string   `json:"access"`
	Network  string   `json:"network"`
	Syntax   string   `json:"syntax"`
	Examples []string `json:"examples"`
}

var commandCatalog = []commandInfo{
	{Name: "about", Category: "Core", Summary: "Show Clyde product details and official links.", Access: "Read-only", Network: "None", Syntax: "clyde --about\nclyde about", Examples: []string{"clyde --about", "clyde about"}},
	{Name: "help", Category: "Core", Summary: "Show human-readable help or a JSON command catalog.", Access: "Read-only", Network: "None", Syntax: "clyde help [--json|COMMAND]", Examples: []string{"clyde help", "clyde help agent", "clyde help --json"}},
	{Name: "completion", Category: "Packaging", Summary: "Generate shell completion for Bash, Zsh, Fish, PowerShell, Elvish, Nushell, Xonsh, Tcsh, Clink, Yash, or Oil.", Access: "Read-only", Network: "None", Syntax: "clyde completion {bash|zsh|fish|powershell|pwsh|elvish|nushell|nu|xonsh|tcsh|clink|yash|oil|osh|ysh}", Examples: []string{"clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde", "clyde completion powershell | Out-String | Invoke-Expression", "clyde completion nushell > clyde-completions.nu"}},
	{Name: "doctor", Category: "Diagnostics", Summary: "Check Clyde configuration, platform, PATH tools, Ollama, and optional repo scan readiness.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde doctor [REPO] [--json] [--ollama-timeout SECONDS]", Examples: []string{"clyde doctor", "clyde doctor . --json"}},
	{Name: "tui", Category: "Interactive", Summary: "Open Clyde's dependency-free terminal UI.", Access: "Local interactive", Network: "Optional local Ollama", Syntax: "clyde tui", Examples: []string{"clyde", "clyde tui"}},
	{Name: "config", Category: "Configuration", Summary: "Show, initialize, or print the Clyde configuration path.", Access: "Reads or writes local config", Network: "None", Syntax: "clyde config {show|init|path}", Examples: []string{"clyde config show", "clyde config init", "clyde config path"}},
	{Name: "config init", Category: "Configuration", Summary: "Write Clyde's default configuration file.", Access: "Writes local config", Network: "None", Syntax: "clyde config init", Examples: []string{"clyde config init"}},
	{Name: "config path", Category: "Configuration", Summary: "Print the config file path Clyde will use.", Access: "Read-only", Network: "None", Syntax: "clyde config path", Examples: []string{"clyde config path"}},
	{Name: "config show", Category: "Configuration", Summary: "Print the effective Clyde configuration as JSON.", Access: "Read-only", Network: "None", Syntax: "clyde config show", Examples: []string{"clyde config show"}},
	{Name: "preview", Category: "Repository Bundles", Summary: "Report included and skipped repository files before bundling or upload.", Access: "Read-only", Network: "None", Syntax: "clyde preview REPO [scan flags] [--json]", Examples: []string{"clyde preview .", "clyde preview . --include 'internal/**/*.go' --json"}},
	{Name: "scan-report", Category: "Repository Bundles", Summary: "Summarize repository scan shape with largest files, extensions, skip reasons, and chunk counts.", Access: "Read-only", Network: "None", Syntax: "clyde scan-report REPO [scan flags] [--json] [--top N]", Examples: []string{"clyde scan-report .", "clyde scan-report . --json", "clyde scan-report . --top 20"}},
	{Name: "bundle", Category: "Repository Bundles", Summary: "Write a digest-bound local manifest and source chunks for review.", Access: "Writes local files", Network: "Optional local subprocess", Syntax: "clyde bundle REPO --out DIR [scan flags] [--force] [--secret-scan-command CMD] [--require-secret-scan]", Examples: []string{"clyde bundle . --out .clyde/out", "clyde bundle . --out .clyde/out --require-secret-scan --secret-scan-command \"gitleaks detect --no-git --source {repo}\""}},
	{Name: "bundle verify", Category: "Repository Bundles", Summary: "Verify a reviewed Clyde bundle and print its upload approval digest.", Access: "Read-only", Network: "None", Syntax: "clyde bundle verify BUNDLE_DIR", Examples: []string{"clyde bundle verify .clyde/out"}},
	{Name: "sync", Category: "NotebookLM Sync", Summary: "Upload a verified bundle, or less-auditable live scan chunks, to NotebookLM after explicit approval.", Access: "Uploads repository chunks", Network: "NotebookLM backend", Syntax: "clyde sync --bundle DIR --approve-digest sha256:... --notebook-id ID --approve-upload [--receipt PATH] [--resume]\nclyde sync REPO --notebook-id ID --approve-upload [--receipt PATH]", Examples: []string{"clyde sync --bundle .clyde/out --approve-digest sha256:... --notebook-id nb --approve-upload", "clyde sync --bundle .clyde/out --approve-digest sha256:... --notebook-id nb --approve-upload --resume", "clyde sync . --notebook-id nb --approve-upload --receipt .clyde/live-sync-receipt.json"}},
	{Name: "daemon", Category: "Status", Summary: "Start the localhost-only JSON-RPC status daemon.", Access: "Starts local server", Network: "Localhost only", Syntax: "clyde daemon [--host HOST] [--port PORT]", Examples: []string{"clyde daemon"}},
	{Name: "status", Category: "Status", Summary: "Read Clyde daemon progress for a job.", Access: "Read-only", Network: "Localhost by default", Syntax: "clyde status [--job-id ID] [--json] [--watch]", Examples: []string{"clyde status", "clyde status --job-id clyde-sync"}},
	{Name: "book", Category: "NotebookLM Sync", Summary: "Generate a dated NotebookLM book title and slug.", Access: "Read-only", Network: "None", Syntax: "clyde book SUBJECT...", Examples: []string{"clyde book Clyde self feedback"}},
	{Name: "models", Category: "Local Models", Summary: "List local Ollama models and the selected default.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde models [--json]", Examples: []string{"clyde models", "clyde models --json"}},
	{Name: "ask", Category: "Local Models", Summary: "Send a direct prompt to the selected local Ollama model.", Access: "Sends prompt to Ollama", Network: "Configured Ollama", Syntax: "clyde ask [flags] PROMPT", Examples: []string{"clyde ask 'Review this function'", "cat prompt.md | clyde ask --stdin"}},
	{Name: "agent", Category: "Local Models", Summary: "Scan repository context and ask a local model for feedback.", Access: "Reads repo and sends selected context", Network: "Local Ollama by default", Syntax: "clyde agent REPO [scan flags] [PROMPT]", Examples: []string{"clyde agent . 'Review this repo'", "clyde agent . --include 'internal/**/*.go' --prompt-file review.md"}},
}
