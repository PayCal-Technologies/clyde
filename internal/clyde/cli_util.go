package clyde

import (
	"flag"
	"io"
	"math"
	"net/url"
	"os"
	"strings"
	"time"
)

type scanFlags struct {
	include       multiFlag
	exclude       multiFlag
	maxFileBytes  int64
	maxChunkChars int
}

func addScanFlags(fs *flag.FlagSet, flags *scanFlags) {
	fs.Var(&flags.include, "include", "only include paths matching this glob; repeatable")
	fs.Var(&flags.exclude, "exclude", "skip paths matching this glob in addition to Clyde defaults; repeatable")
	fs.Int64Var(&flags.maxFileBytes, "max-file-bytes", flags.maxFileBytes, "skip files larger than this many bytes")
	fs.IntVar(&flags.maxChunkChars, "max-chunk-chars", flags.maxChunkChars, "split uploaded source text at this many characters")
}

func scanAndChunk(repo string, flags scanFlags, bookTitle string) (ScanResult, []ChunkRecord, error) {
	result, err := ScanRepo(repo, flags.include, flags.exclude, flags.maxFileBytes)
	if err != nil {
		return ScanResult{}, nil, err
	}
	return result, MakeChunks(result, flags.maxChunkChars, bookTitle), nil
}

func planFromArgs(subject, bookTitle string) (BookPlan, error) {
	if bookTitle != "" {
		return BookPlanFromTitle(bookTitle)
	}
	return NewBookPlan(subject, time.Now())
}

func validateScanFlags(flags scanFlags) error {
	if flags.maxFileBytes <= 0 {
		return errf("max-file-bytes must be greater than 0")
	}
	if flags.maxChunkChars <= 0 {
		return errf("max-chunk-chars must be greater than 0")
	}
	return nil
}

func validatePositiveSeconds(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return errf("--%s must be greater than 0", name)
	}
	if value > maxCLISeconds {
		return errf("--%s is too large; maximum is %d seconds", name, maxCLISeconds)
	}
	return nil
}

func validateNumCtxFlag(value int) error {
	if value < 0 {
		return errf("--num-ctx must be zero or greater")
	}
	if value > maxConfigNumCtx {
		return errf("--num-ctx is too large; maximum is %d", maxConfigNumCtx)
	}
	return nil
}

func validateCommandFlag(name, value string) error {
	if strings.ContainsRune(value, 0) {
		return errf("--%s must not contain NUL bytes", name)
	}
	if len(shellFields(value)) == 0 {
		return errf("--%s must not be empty", name)
	}
	return nil
}

func isHelpArgs(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errf("must not be empty")
	}
	*m = append(*m, value)
	return nil
}

func shellFields(value string) []string {
	return strings.Fields(value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func promptText(stdin io.Reader, args []string, promptFile string, readStdin bool) (string, error) {
	if promptFile != "" && readStdin {
		return "", errf("--prompt-file and --stdin are mutually exclusive")
	}
	if promptFile != "" {
		file, err := os.Open(promptFile)
		if err != nil {
			return "", err
		}
		defer file.Close()
		data, err := readLimitedText(file, maxPromptInputBytes, "--prompt-file")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if readStdin {
		data, err := readLimitedText(stdin, maxPromptInputBytes, "--stdin")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return strings.Join(args, " "), nil
}

func readLimitedText(reader io.Reader, limit int64, label string) ([]byte, error) {
	limited := io.LimitReader(reader, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errf("%s input is too large; maximum is %d bytes", label, limit)
	}
	return data, nil
}

func isLocalOllamaURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func interspersedArgs(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if name == "h" || name == "help" {
			continue
		}
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			continue
		}
		if boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}
