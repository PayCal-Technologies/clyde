package clyde

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func cmdConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		args = []string{"show"}
	}
	if isHelpArgs(args) {
		printConfigHelp(out)
		return flag.ErrHelp
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(out, path)
		return nil
	case "show":
		cfg, path, err := LoadConfig()
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(map[string]any{
			"path":   path,
			"config": cfg,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	case "init":
		if _, err := os.Lstat(path); err == nil {
			return errf("config already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := WriteDefaultConfig(path); err != nil {
			return err
		}
		fmt.Fprintf(out, "Wrote: %s\n", path)
		return nil
	default:
		return errf("unknown config command: %s", args[0])
	}
}
