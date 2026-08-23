package clyde

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"
)

func cmdDaemon(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(out)
	host := fs.String("host", "127.0.0.1", "host")
	port := fs.Int("port", 5876, "port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *port <= 0 || *port > 65535 {
		return errf("--port must be between 1 and 65535")
	}
	return ServeStatus(*host, *port, out)
}

func cmdStatus(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(out)
	host := fs.String("host", "127.0.0.1", "host")
	port := fs.Int("port", 5876, "port")
	jobID := fs.String("job-id", "", "job id")
	jsonOut := fs.Bool("json", false, "print raw JSON")
	watch := fs.Bool("watch", false, "poll until terminal")
	interval := fs.Float64("interval", 1, "poll interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *port <= 0 || *port > 65535 {
		return errf("--port must be between 1 and 65535")
	}
	if err := validatePositiveSeconds("interval", *interval); err != nil {
		return err
	}
	seen := ""
	for {
		result, err := FetchStatus(StatusURL(*host, *port), *jobID)
		if err != nil {
			return err
		}
		rendered := FormatStatus(result)
		if *jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			rendered = string(data)
		}
		if rendered != seen {
			fmt.Fprintln(out, rendered)
			seen = rendered
		}
		if !*watch || terminalStatus(result) {
			return nil
		}
		time.Sleep(time.Duration(*interval * float64(time.Second)))
	}
}
