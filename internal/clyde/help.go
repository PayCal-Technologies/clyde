package clyde

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func printHelp(out io.Writer) {
	fmt.Fprintf(out, "usage: clyde {%s} ...\n", topLevelCommandUsageList())
	fmt.Fprintln(out, "run clyde with no arguments in a terminal to open the TUI")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde help agent")
	fmt.Fprintln(out, "  clyde help --json")
	fmt.Fprintln(out, "  clyde --about")
	fmt.Fprintln(out, "  clyde preview . --include 'internal/**/*.go'")
	fmt.Fprintln(out, "  clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde")
	fmt.Fprintln(out, "  clyde completion powershell | Out-String | Invoke-Expression")
	fmt.Fprintln(out, "  clyde completion nushell > clyde-completions.nu")
	fmt.Fprintln(out, "  clyde doctor . --json")
	fmt.Fprintln(out, "  clyde scan-report . --json")
	fmt.Fprintln(out, "  clyde models")
	fmt.Fprintln(out, "  clyde config show")
	fmt.Fprintln(out, "  clyde ask --model qwen2.5-coder:7b --stdin")
	fmt.Fprintln(out, "  clyde agent . --model qwen2.5-coder:7b 'review this repo'")
	fmt.Fprintln(out)
	printLinks(out)
	fmt.Fprintln(out, "run `clyde --about` for product details")
}

func cmdHelp(args []string, stdin io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		printHelp(out)
		return nil
	}
	if len(args) == 1 && args[0] == "--json" {
		data, err := json.MarshalIndent(map[string]any{
			"product":  productName,
			"version":  productVersion,
			"home":     productHomeURL,
			"help":     productHelpURL,
			"github":   productGitHubURL,
			"commands": commandCatalog(),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	target := strings.Join(args, " ")
	if info, ok := commandByName(target); ok && strings.Contains(target, " ") {
		printCommandInfo(out, info)
		return nil
	}
	if len(args) != 1 {
		return errf("help accepts one command, or one command plus a subcommand")
	}
	switch target {
	case "about":
		printAbout(out)
		return nil
	case "help":
		printHelpCommand(out)
		return nil
	case "completion":
		printCompletionHelp(out)
		return nil
	case "doctor":
		printDoctorHelp(out)
		return nil
	case "scan-report":
		printScanReportHelp(out)
		return nil
	case "config":
		printConfigHelp(out)
		return nil
	case "tui":
		printTUIHelp(out)
		return nil
	case "book":
		printBookHelp(out)
		return nil
	case "receipt":
		printReceiptHelp(out)
		return nil
	}
	return run([]string{target, "--help"}, stdin, out, errOut)
}

func printHelpCommand(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde help [--json|command]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Show Clyde's top-level help, command-specific help, or a JSON command catalog.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde help")
	fmt.Fprintln(out, "  clyde help agent")
	fmt.Fprintln(out, "  clyde help --json")
}

func printCompletionHelp(out io.Writer) {
	fmt.Fprintf(out, "usage: clyde completion {%s}\n", supportedShellChoiceList())
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Generate shell completion scripts for Clyde.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde")
	fmt.Fprintln(out, "  clyde completion bash > ~/.clyde-completion.bash")
	fmt.Fprintln(out, "  clyde completion fish > ~/.config/fish/completions/clyde.fish")
	fmt.Fprintln(out, "  clyde completion powershell | Out-String | Invoke-Expression")
	fmt.Fprintln(out, "  clyde completion elvish > ~/.elvish/completions/clyde.elv")
	fmt.Fprintln(out, "  clyde completion nushell > clyde-completions.nu")
	fmt.Fprintln(out, "  clyde completion xonsh > ~/.xonshrc.d/clyde-completions.xsh")
	fmt.Fprintln(out, "  clyde completion tcsh >> ~/.tcshrc")
	fmt.Fprintln(out, "  clyde completion clink > %LOCALAPPDATA%\\clink\\clyde.lua")
	fmt.Fprintln(out, "  clyde completion yash >> ~/.yashrc")
	fmt.Fprintln(out, "  clyde completion oil > ~/.config/oils/clyde-completions.sh")
}

func printDoctorHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde doctor [repo] [--json] [--ollama-timeout seconds]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Check Clyde's local environment without uploading data.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde doctor")
	fmt.Fprintln(out, "  clyde doctor .")
	fmt.Fprintln(out, "  clyde doctor . --json")
}

func printScanReportHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde scan-report REPO [scan flags] [--json] [--top N]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summarize repository scan shape without writing bundles or uploading data.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde scan-report .")
	fmt.Fprintln(out, "  clyde scan-report . --json")
	fmt.Fprintln(out, "  clyde scan-report . --include \"internal/**/*.go\" --top 20")
}

func printConfigHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde config {show|init|path}")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Manage Clyde's JSON configuration file.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde config show")
	fmt.Fprintln(out, "  clyde config init")
	fmt.Fprintln(out, "  clyde config path")
}

func printTUIHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde tui")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Open Clyde's dependency-free terminal UI.")
	fmt.Fprintln(out, "Running `clyde` with no arguments in an interactive terminal also opens the TUI.")
}

func printBookHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde book SUBJECT...")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Generate a dated NotebookLM book title and slug.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "example:")
	fmt.Fprintln(out, "  clyde book Clyde self feedback")
}

func printReceiptHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde receipt status PATH [--json]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Show read-only sync receipt state and safe resume guidance.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde receipt status .clyde/out/sync-receipt.json")
	fmt.Fprintln(out, "  clyde receipt status .clyde/out/sync-receipt.json --json")
}

func printCommandInfo(out io.Writer, command commandInfo) {
	fmt.Fprintf(out, "usage: %s\n", command.Syntax)
	fmt.Fprintln(out)
	fmt.Fprintln(out, command.Summary)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "category: %s\n", command.Category)
	fmt.Fprintf(out, "access: %s\n", command.Access)
	fmt.Fprintf(out, "network: %s\n", command.Network)
	if len(command.Examples) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "examples:")
		for _, example := range command.Examples {
			fmt.Fprintf(out, "  %s\n", example)
		}
	}
}

func printAbout(out io.Writer) {
	fmt.Fprintf(out, "%s %s\n", productName, productVersion)
	fmt.Fprintln(out, productDescription)
	fmt.Fprintf(out, "Created by %s\n", productCreator)
	fmt.Fprintln(out)
	printLinks(out)
}

func printLinks(out io.Writer) {
	fmt.Fprintf(out, "Home: %s\n", productHomeURL)
	fmt.Fprintf(out, "Help: %s\n", productHelpURL)
	fmt.Fprintf(out, "GitHub: %s\n", productGitHubURL)
	fmt.Fprintf(out, "PayCal Technologies: %s\n", productCreatorURL)
}
