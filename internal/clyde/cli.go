package clyde

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	productName        = "Clyde"
	productVersion     = "0.2.5"
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
		return 1
	}
	return 0
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	handler, ok := commandHandler(args[0])
	if !ok {
		return errf("unknown command: %s", args[0])
	}
	return handler(args[1:], stdin, stdout, stderr)
}
