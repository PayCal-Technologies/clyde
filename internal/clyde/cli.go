package clyde

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	productName        = "Clyde"
	productVersion     = "1.0.2"
	productDescription = "local repository review, bundling, and NotebookLM sync harness"
	productHomeURL     = "https://paycaltech.com/clyde"
	productHelpURL     = "https://paycaltech.com/clyde/help"
	productGitHubURL   = "https://github.com/PayCal-Technologies/clyde"
	productCreator     = "PayCal Technologies"
	productCreatorURL  = "https://paycaltech.com"
)

const maxCLISeconds = 24 * 60 * 60

const maxPromptInputBytes = 1 << 20

func Main(args []string, stdout, stderr io.Writer) int {
	return MainWithInput(args, os.Stdin, stdout, stderr)
}

func MainWithInput(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if file, ok := stdin.(*os.File); ok {
			if stat, err := file.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
				if err := RunTUI(stdin, stdout, stderr); err != nil {
					fmt.Fprintf(stderr, "clyde: error: %v\n", err)
					return 1
				}
				return 0
			}
		}
		printHelp(stdout)
		return 0
	}
	if args[0] == "-h" || args[0] == "--help" {
		printHelp(stdout)
		return 0
	}
	if args[0] == "--about" || args[0] == "about" {
		printAbout(stdout)
		return 0
	}
	if err := run(args, stdin, stdout, stderr); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "clyde: error: %v\n", err)
		if hint := nextActionHint(args, err.Error()); hint != "" {
			fmt.Fprintf(stderr, "clyde: next: %s\n", hint)
		}
		return 1
	}
	return 0
}

func nextActionHint(args []string, message string) string {
	switch {
	case strings.HasPrefix(message, "unknown command:"):
		return "run `clyde help` to list commands."
	case strings.Contains(message, "requires REPO"):
		command := "preview"
		if len(args) > 0 && args[0] != "" {
			command = args[0]
		}
		return fmt.Sprintf("choose a repository path, for example: `clyde %s .`.", command)
	case strings.Contains(message, "requires --approve-upload"):
		return "run the command with `--dry-run` first; add `--approve-upload` only after review."
	case strings.Contains(message, "requires exactly one of --notebook-id or --notebook-url"):
		return "provide one destination, for example: `--notebook-id YOUR_NOTEBOOK_ID`."
	case strings.Contains(message, "requires --approve-digest"):
		return "run `clyde bundle verify BUNDLE_DIR`, then copy its printed digest into `--approve-digest`."
	case strings.Contains(message, "--resume requires --receipt"):
		return "provide the receipt from the original sync with `--receipt PATH`."
	case strings.Contains(message, "--resume requires an existing sync receipt"):
		return "use the receipt created by the original sync, or remove `--resume` to start a new transfer."
	default:
		return "run `clyde help " + helpTarget(args) + "` for command usage."
	}
}

func helpTarget(args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	return ""
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	handler, ok := commandHandler(args[0])
	if !ok {
		return errf("unknown command: %s", args[0])
	}
	return handler(args[1:], stdin, stdout, stderr)
}
