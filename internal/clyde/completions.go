package clyde

import (
	"flag"
	"io"
)

const bashCompletionScript = `# bash completion for clyde
_clyde_completion() {
  local cur prev commands
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  commands="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh" -- "${cur}") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json" -- "${cur}") )
      ;;
    doctor)
      COMPREPLY=( $(compgen -W "--json --ollama-timeout --help" -- "${cur}") )
      ;;
    config)
      COMPREPLY=( $(compgen -W "path show init" -- "${cur}") )
      ;;
    preview)
      COMPREPLY=( $(compgen -W "--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --show-files --show-skips --json --help" -- "${cur}") )
      ;;
    scan-report)
      COMPREPLY=( $(compgen -W "--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --json --top --help" -- "${cur}") )
      ;;
    bundle)
      COMPREPLY=( $(compgen -W "verify --out --subject --book-title --force --secret-scan-command --require-secret-scan --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help" -- "${cur}") )
      ;;
    sync)
      COMPREPLY=( $(compgen -W "--notebook-id --notebook-url --approve-upload --bundle --approve-digest --receipt --resume --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help" -- "${cur}") )
      ;;
    models)
      COMPREPLY=( $(compgen -W "--ollama-url --timeout --json --help" -- "${cur}") )
      ;;
    ask)
      COMPREPLY=( $(compgen -W "--model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --help" -- "${cur}") )
      ;;
    agent)
      COMPREPLY=( $(compgen -W "--model --ollama-url --timeout --max-context-chars --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help" -- "${cur}") )
      ;;
    daemon)
      COMPREPLY=( $(compgen -W "--host --port --help" -- "${cur}") )
      ;;
    status)
      COMPREPLY=( $(compgen -W "--host --port --job-id --json --watch --interval --help" -- "${cur}") )
      ;;
  esac
}
complete -F _clyde_completion clyde
`

const zshCompletionScript = `#compdef clyde
_clyde() {
  local -a commands
  commands=(
    'about:show product details and links'
    'help:show command help or JSON command catalog'
    'completion:print shell completion script'
    'doctor:check local Clyde environment'
    'tui:open the terminal UI'
    'config:manage Clyde config'
    'preview:show files Clyde would scan'
    'bundle:write or verify digest-bound manifest.json and chunks.jsonl'
    'sync:upload verified bundle chunks to NotebookLM'
    'daemon:serve sync status'
    'status:read sync status'
    'book:plan a dated NotebookLM book name'
    'models:list local Ollama models'
    'ask:ask a local Ollama model'
    'agent:scan repo and ask a local Ollama model'
  )

  _arguments -C \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        completion) _values 'shell' bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh ;;
        help) _values 'command' about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json ;;
        doctor) _arguments '--json' '--ollama-timeout=[]' '--help' ;;
        config) _values 'config command' path show init ;;
        preview) _arguments '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--allow-filesystem-fallback' '--show-files=[]' '--show-skips=[]' '--json' '--help' ;;
        scan-report) _arguments '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--allow-filesystem-fallback' '--json' '--top=[]' '--help' ;;
        bundle) _arguments '1:subcommand:(verify)' '--out=[]' '--subject=[]' '--book-title=[]' '--force' '--secret-scan-command=[]' '--require-secret-scan' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--allow-filesystem-fallback' '--help' ;;
        sync) _arguments '--notebook-id=[]' '--notebook-url=[]' '--approve-upload' '--bundle=[]' '--approve-digest=[]' '--receipt=[]' '--resume' '--backend=[mcp or nlm]:backend:(mcp nlm)' '--mcp-command=[]' '--nlm-command=[]' '--delete-existing-sources' '--mcp-timeout=[]' '--status-url=[]' '--quiet-progress' '--job-id=[]' '--subject=[]' '--book-title=[]' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--allow-filesystem-fallback' '--help' ;;
        models) _arguments '--ollama-url=[]' '--timeout=[]' '--json' '--help' ;;
        ask) _arguments '--model=[]' '--ollama-url=[]' '--timeout=[]' '--num-ctx=[]' '--no-stream' '--prompt-file=[]' '--stdin' '--help' ;;
        agent) _arguments '--model=[]' '--ollama-url=[]' '--timeout=[]' '--max-context-chars=[]' '--num-ctx=[]' '--no-stream' '--prompt-file=[]' '--stdin' '--allow-remote-ollama' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--allow-filesystem-fallback' '--help' ;;
        daemon) _arguments '--host=[]' '--port=[]' '--help' ;;
        status) _arguments '--host=[]' '--port=[]' '--job-id=[]' '--json' '--watch' '--interval=[]' '--help' ;;
      esac
      ;;
  esac
}
_clyde "$@"
`

const fishCompletionScript = `# fish completion for clyde
complete -c clyde -f
complete -c clyde -n "__fish_use_subcommand" -a "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent"
complete -c clyde -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh"
complete -c clyde -n "__fish_seen_subcommand_from help" -a "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json"
complete -c clyde -n "__fish_seen_subcommand_from doctor" -l json
complete -c clyde -n "__fish_seen_subcommand_from doctor" -l ollama-timeout -r
complete -c clyde -n "__fish_seen_subcommand_from config" -a "path show init"
complete -c clyde -n "__fish_seen_subcommand_from preview" -l include -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l exclude -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-file-bytes -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-chunk-chars -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l allow-filesystem-fallback
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-files -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-skips -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l json
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l include -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l exclude -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l max-file-bytes -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l max-chunk-chars -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l allow-filesystem-fallback
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l json
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l top -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l out -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l subject -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l book-title -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l force
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l secret-scan-command -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l require-secret-scan
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l allow-filesystem-fallback
complete -c clyde -n "__fish_seen_subcommand_from sync" -l notebook-id -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l notebook-url -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l approve-upload
complete -c clyde -n "__fish_seen_subcommand_from sync" -l bundle -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l approve-digest -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l receipt -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l resume
complete -c clyde -n "__fish_seen_subcommand_from sync" -l backend -xa "mcp nlm"
complete -c clyde -n "__fish_seen_subcommand_from sync" -l delete-existing-sources
complete -c clyde -n "__fish_seen_subcommand_from sync" -l allow-filesystem-fallback
complete -c clyde -n "__fish_seen_subcommand_from models" -l ollama-url -r
complete -c clyde -n "__fish_seen_subcommand_from models" -l timeout -r
complete -c clyde -n "__fish_seen_subcommand_from models" -l json
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l model -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l ollama-url -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l timeout -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l num-ctx -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l no-stream
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l prompt-file -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l stdin
complete -c clyde -n "__fish_seen_subcommand_from agent" -l allow-remote-ollama
complete -c clyde -n "__fish_seen_subcommand_from agent" -l allow-filesystem-fallback
complete -c clyde -n "__fish_seen_subcommand_from agent" -l max-context-chars -r
complete -c clyde -n "__fish_seen_subcommand_from daemon status" -l host -r
complete -c clyde -n "__fish_seen_subcommand_from daemon status" -l port -r
complete -c clyde -n "__fish_seen_subcommand_from status" -l watch
complete -c clyde -n "__fish_seen_subcommand_from status" -l interval -r
`

const powerShellCompletionScript = `# PowerShell completion for clyde
Register-ArgumentCompleter -Native -CommandName clyde -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)

  $commands = @(
    'about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle',
    'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent'
  )
  $commandDescriptions = @{
    about = 'show product details and links'
    help = 'show command help or JSON command catalog'
    completion = 'print shell completion script'
    doctor = 'check local Clyde environment'
    tui = 'open the terminal UI'
    config = 'manage Clyde config'
    preview = 'show files Clyde would scan'
    bundle = 'write or verify digest-bound manifest.json and chunks.jsonl'
    sync = 'upload verified bundle chunks to NotebookLM'
    daemon = 'serve sync status'
    status = 'read sync status'
    book = 'plan a dated NotebookLM book name'
    models = 'list local Ollama models'
    ask = 'ask a local Ollama model'
    agent = 'scan repo and ask a local Ollama model'
  }
  $subcommands = @{
		completion = @('bash', 'zsh', 'fish', 'powershell', 'pwsh', 'elvish', 'nushell', 'nu', 'xonsh', 'tcsh', 'clink', 'yash', 'oil', 'osh', 'ysh')
    help = @('about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle', 'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent', '--json')
    config = @('path', 'show', 'init')
  }
  $flags = @{
    preview = @('--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--show-files', '--show-skips', '--json', '--help')
    'scan-report' = @('--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--json', '--top', '--help')
    doctor = @('--json', '--ollama-timeout', '--help')
    bundle = @('verify', '--out', '--subject', '--book-title', '--force', '--secret-scan-command', '--require-secret-scan', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help')
    sync = @('--notebook-id', '--notebook-url', '--approve-upload', '--bundle', '--approve-digest', '--receipt', '--resume', '--backend', '--mcp-command', '--nlm-command', '--delete-existing-sources', '--mcp-timeout', '--status-url', '--quiet-progress', '--job-id', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help')
    models = @('--ollama-url', '--timeout', '--json', '--help')
    ask = @('--model', '--ollama-url', '--timeout', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--help')
    agent = @('--model', '--ollama-url', '--timeout', '--max-context-chars', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--allow-remote-ollama', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help')
    daemon = @('--host', '--port', '--help')
    status = @('--host', '--port', '--job-id', '--json', '--watch', '--interval', '--help')
  }
  $flagValues = @{
    '--backend' = @('mcp', 'nlm')
  }

  $words = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
  if ($words.Count -le 1) {
    $candidates = $commands
  } else {
    $command = $words[1]
    $previous = if ($words.Count -gt 1) { $words[$words.Count - 2] } else { '' }
    if ($flagValues.ContainsKey($previous)) {
      $candidates = $flagValues[$previous]
    } elseif ($subcommands.ContainsKey($command)) {
      $candidates = $subcommands[$command]
    } elseif ($flags.ContainsKey($command)) {
      $candidates = $flags[$command]
    } else {
      $candidates = @()
    }
  }

  $candidates |
    Where-Object { $_ -like "$wordToComplete*" } |
    ForEach-Object {
      $description = if ($commandDescriptions.ContainsKey($_)) { $commandDescriptions[$_] } else { $_ }
      [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $description)
    }
}
`

const elvishCompletionScript = `# Elvish completion for clyde
edit:completion:arg-completer[clyde] = [@words]{
  var commands = [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent]
  var shells = [bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh]
  var config-subcommands = [path show init]
  var common-scan-flags = [--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback]
  var flags = [
    &preview=[--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --show-files --show-skips --json --help]
    &scan-report=[--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --json --top --help]
    &doctor=[--json --ollama-timeout --help]
    &bundle=[verify --out --subject --book-title --force --secret-scan-command --require-secret-scan --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help]
    &sync=[--notebook-id --notebook-url --approve-upload --bundle --approve-digest --receipt --resume --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help]
    &models=[--ollama-url --timeout --json --help]
    &ask=[--model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --help]
    &agent=[--model --ollama-url --timeout --max-context-chars --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --help]
    &daemon=[--host --port --help]
    &status=[--host --port --job-id --json --watch --interval --help]
  ]
  var stem = $words[-1]
  var choices = []
  if (== (count $words) 2) {
    set choices = $commands
  } elif (== $words[1] completion) {
    set choices = $shells
  } elif (== $words[1] help) {
    set choices = [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json]
  } elif (== $words[1] config) {
    set choices = $config-subcommands
  } elif (has-key $flags $words[1]) {
    if (== $words[-2] --backend) {
      set choices = [mcp nlm]
    } else {
      set choices = $flags[$words[1]]
    }
  } else {
    set choices = $common-scan-flags
  }
  put $@choices | each [choice]{ if (has-prefix $choice $stem) { put $choice } }
}
`

const nushellCompletionScript = `# Nushell completion for clyde
def "nu-complete clyde commands" [] {
  [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent]
}

def "nu-complete clyde shells" [] {
  [bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh]
}

def "nu-complete clyde config" [] {
  [path show init]
}

def "nu-complete clyde backend" [] {
  [mcp nlm]
}

extern "clyde" [
  command?: string@"nu-complete clyde commands"
  subcommand?: string
  --include: string
  --exclude: string
  --max-file-bytes: int
  --max-chunk-chars: int
  --show-files: int
  --show-skips: int
  --top: int
  --json
  --ollama-timeout: int
  --out: string
  --subject: string
  --book-title: string
  --notebook-id: string
  --notebook-url: string
  --approve-upload
  --backend: string@"nu-complete clyde backend"
  --mcp-command: string
  --nlm-command: string
  --delete-existing-sources
  --mcp-timeout: int
  --status-url: string
  --quiet-progress
  --job-id: string
  --model: string
  --ollama-url: string
  --timeout: int
  --num-ctx: int
  --no-stream
  --prompt-file: string
  --stdin
  --allow-remote-ollama
  --host: string
  --port: int
  --watch
  --interval: int
  --help
]
`

const xonshCompletionScript = `# Xonsh completion for clyde
from xonsh.completers.completer import add_one_completer

_CLYDE_COMMANDS = {
    'about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle',
    'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent',
}
_CLYDE_SHELLS = {
    'bash', 'zsh', 'fish', 'powershell', 'pwsh', 'elvish', 'nushell', 'nu',
    'xonsh', 'tcsh', 'clink', 'yash', 'oil', 'osh', 'ysh',
}
_CLYDE_FLAGS = {
    'preview': {'--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--show-files', '--show-skips', '--json', '--help'},
    'scan-report': {'--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--json', '--top', '--help'},
    'doctor': {'--json', '--ollama-timeout', '--help'},
    'bundle': {'--out', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help'},
    'sync': {'--notebook-id', '--notebook-url', '--approve-upload', '--backend', '--mcp-command', '--nlm-command', '--delete-existing-sources', '--mcp-timeout', '--status-url', '--quiet-progress', '--job-id', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help'},
    'models': {'--ollama-url', '--timeout', '--json', '--help'},
    'ask': {'--model', '--ollama-url', '--timeout', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--help'},
    'agent': {'--model', '--ollama-url', '--timeout', '--max-context-chars', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--allow-remote-ollama', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--allow-filesystem-fallback', '--help'},
    'daemon': {'--host', '--port', '--help'},
    'status': {'--host', '--port', '--job-id', '--json', '--watch', '--interval', '--help'},
}

def _clyde_completer(prefix, line, begidx, endidx, ctx):
    parts = line[:endidx].split()
    if len(parts) <= 1:
        candidates = _CLYDE_COMMANDS
    else:
        command = parts[1]
        previous = parts[-2] if len(parts) > 1 else ''
        if previous == '--backend':
            candidates = {'mcp', 'nlm'}
        elif command == 'completion':
            candidates = _CLYDE_SHELLS
        elif command == 'help':
            candidates = _CLYDE_COMMANDS | {'--json'}
        elif command == 'config':
            candidates = {'path', 'show', 'init'}
        else:
            candidates = _CLYDE_FLAGS.get(command, set())
    return {candidate for candidate in candidates if candidate.startswith(prefix)}

add_one_completer('clyde', _clyde_completer, 'start')
`

const tcshCompletionScript = `# tcsh completion for clyde
complete clyde \
  'p/1/(about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent)/' \
  'n/completion/(bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh)/' \
  'n/help/(about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json)/' \
  'n/config/(path show init)/' \
  'n/--backend/(mcp nlm)/' \
  'c/--/(--include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --show-files --show-skips --json --top --ollama-timeout --out --subject --book-title --force --secret-scan-command --require-secret-scan --notebook-id --notebook-url --approve-upload --bundle --approve-digest --receipt --resume --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --host --port --watch --interval --help)/'
`

const clinkCompletionScript = `-- Clink completion for clyde
local commands = {
  "about", "help", "completion", "doctor", "tui", "config", "preview", "scan-report", "bundle",
  "sync", "daemon", "status", "book", "models", "ask", "agent"
}
local shells = {
  "bash", "zsh", "fish", "powershell", "pwsh", "elvish", "nushell", "nu",
  "xonsh", "tcsh", "clink", "yash", "oil", "osh", "ysh"
}
local flags = {
  "--include", "--exclude", "--max-file-bytes", "--max-chunk-chars", "--allow-filesystem-fallback",
  "--show-files", "--show-skips", "--json", "--top", "--out", "--subject",
  "--book-title", "--notebook-id", "--notebook-url", "--approve-upload",
  "--backend", "--mcp-command", "--nlm-command", "--delete-existing-sources",
  "--mcp-timeout", "--status-url", "--quiet-progress", "--job-id", "--model",
  "--ollama-url", "--timeout", "--num-ctx", "--no-stream", "--prompt-file",
  "--stdin", "--allow-remote-ollama", "--host", "--port", "--watch",
  "--interval", "--ollama-timeout", "--help"
}

local parser = clink.argmatcher("clyde")
parser:addarg(commands)
parser:addarg({
  fromhistory = false,
  function(word, word_index, line_state, match_builder)
    local command = line_state:getword(2)
    local previous = line_state:getword(word_index - 1)
    local choices = flags
    if previous == "--backend" then
      choices = {"mcp", "nlm"}
    elseif command == "completion" then
      choices = shells
    elseif command == "help" then
      choices = {"about", "help", "completion", "doctor", "tui", "config", "preview", "scan-report", "bundle", "sync", "daemon", "status", "book", "models", "ask", "agent", "--json"}
    elseif command == "config" then
      choices = {"path", "show", "init"}
    end
    for _, choice in ipairs(choices) do
      if choice:sub(1, #word) == word then
        match_builder:addmatch(choice)
      end
    end
    return true
  end
})
`

const yashCompletionScript = `# Yash completion for clyde
function completion//argument/clyde {
  local words="${COMP_WORDS[*]}"
  local current="${COMP_WORDS[$COMP_CWORD]}"
  local command="${COMP_WORDS[1]}"
  local previous="${COMP_WORDS[$((COMP_CWORD - 1))]}"
  local candidates
  case "$command:$previous" in
    completion:*) candidates="bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh" ;;
    help:*) candidates="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json" ;;
    config:*) candidates="path show init" ;;
    *:--backend) candidates="mcp nlm" ;;
    *) candidates="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --include --exclude --max-file-bytes --max-chunk-chars --allow-filesystem-fallback --show-files --show-skips --json --top --ollama-timeout --out --subject --book-title --force --secret-scan-command --require-secret-scan --notebook-id --notebook-url --approve-upload --bundle --approve-digest --receipt --resume --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --host --port --watch --interval --help" ;;
  esac
  for candidate in $candidates; do
    case "$candidate" in
      "$current"*) printf '%s\n' "$candidate" ;;
    esac
  done
}
`

const oilCompletionScript = `# Oil/OSH/YSH completion for clyde
# OSH runs Bash completion scripts, so Clyde uses its Bash completer for Oil-family shells.
` + bashCompletionScript

func cmdCompletion(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printCompletionHelp(out)
		return flag.ErrHelp
	}
	if len(args) != 1 {
		return errf("completion requires shell: %s", supportedShellList())
	}
	script, ok := completionScriptForShell(args[0])
	if !ok {
		return errf("unsupported shell %q; expected %s", args[0], supportedShellList())
	}
	io.WriteString(out, script)
	return nil
}
