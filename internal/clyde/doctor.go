package clyde

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorReport struct {
	Product string        `json:"product"`
	Version string        `json:"version"`
	OS      string        `json:"os"`
	Arch    string        `json:"arch"`
	Checks  []doctorCheck `json:"checks"`
}

func doctorExecutableCheck(name, purpose string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorCheck{Name: name, Status: "warn", Message: name + " not found on PATH; " + purpose + " may be unavailable"}
	}
	return doctorCheck{Name: name, Status: "ok", Message: path}
}

func doctorOllamaCheck(cfg Config, timeout time.Duration) doctorCheck {
	if err := validateOllamaURL(cfg.OllamaURL); err != nil {
		return doctorCheck{Name: "ollama", Status: "error", Message: err.Error()}
	}
	client := NewOllamaClient(cfg.OllamaURL, timeout)
	models, err := client.ListModels(context.Background())
	if err != nil {
		return doctorCheck{Name: "ollama", Status: "warn", Message: err.Error()}
	}
	if len(models) == 0 {
		return doctorCheck{Name: "ollama", Status: "warn", Message: "reachable at " + client.BaseURL + " but no models are installed"}
	}
	selected, _ := SelectModel("", cfg, models)
	return doctorCheck{Name: "ollama", Status: "ok", Message: "reachable at " + client.BaseURL + "; selected model " + selected}
}

func doctorRepoCheck(repo string, cfg Config) doctorCheck {
	result, err := ScanRepoWithOptions(repo, ScanOptions{
		ExcludeFolders: cfg.ExcludeFolders,
		MaxFileBytes:   cfg.MaxFileBytes,
	})
	if err != nil {
		return doctorCheck{Name: "repo", Status: "error", Message: err.Error()}
	}
	chunks := MakeChunks(result, cfg.MaxChunkChars, "")
	status := "ok"
	if len(result.Files) == 0 {
		status = "warn"
	}
	return doctorCheck{
		Name:   "repo",
		Status: status,
		Message: fmt.Sprintf("%d files, %d skips, %d chunks, %s total",
			len(result.Files), len(result.Skips), len(chunks), formatBytes(result.TotalBytes())),
	}
}

func printDoctorReport(out io.Writer, report doctorReport) {
	fmt.Fprintf(out, "%s doctor %s (%s/%s)\n", report.Product, report.Version, report.OS, report.Arch)
	for _, check := range report.Checks {
		fmt.Fprintf(out, "%s %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
}

func doctorResultError(report doctorReport) error {
	for _, check := range report.Checks {
		if check.Status == "error" {
			return errf("doctor found errors")
		}
	}
	return nil
}

func cmdDoctor(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printDoctorHelp(out)
		return flag.ErrHelp
	}
	cfg := DefaultConfig()
	configPath, pathErr := ConfigPath()
	loaded, loadedPath, configErr := LoadConfig()
	if configErr == nil {
		cfg = loaded
		if loadedPath != "" {
			configPath = loadedPath
		}
	}
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(out)
	jsonOut := fs.Bool("json", false, "print machine-readable diagnostics")
	ollamaTimeout := fs.Float64("ollama-timeout", 2, "seconds to wait for local Ollama diagnostics")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errf("doctor accepts at most one optional REPO")
	}
	if err := validatePositiveSeconds("ollama-timeout", *ollamaTimeout); err != nil {
		return err
	}
	report := doctorReport{
		Product: productName,
		Version: productVersion,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
	report.Checks = append(report.Checks, doctorCheck{Name: "version", Status: "ok", Message: productName + " " + productVersion})
	report.Checks = append(report.Checks, doctorCheck{Name: "platform", Status: "ok", Message: runtime.GOOS + "/" + runtime.GOARCH})
	if pathErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "error", Message: pathErr.Error()})
	} else if configErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "error", Message: configPath + ": " + configErr.Error()})
	} else if _, err := os.Stat(configPath); os.IsNotExist(err) {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "warn", Message: "no config file at " + configPath + "; using defaults"})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "ok", Message: "loaded " + configPath})
	}
	report.Checks = append(report.Checks, doctorExecutableCheck("git", "fast repository file discovery"))
	report.Checks = append(report.Checks, doctorExecutableCheck("npx", "default NotebookLM MCP backend"))
	report.Checks = append(report.Checks, doctorExecutableCheck("nlm", "optional NotebookLM CLI backend"))
	report.Checks = append(report.Checks, doctorOllamaCheck(cfg, time.Duration(*ollamaTimeout*float64(time.Second))))
	if fs.NArg() == 1 {
		report.Checks = append(report.Checks, doctorRepoCheck(fs.Arg(0), cfg))
	}
	if *jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return doctorResultError(report)
	}
	printDoctorReport(out, report)
	return doctorResultError(report)
}
