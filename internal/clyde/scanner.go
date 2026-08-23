package clyde

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxGitListBytes           = 4 * 1024 * 1024
	gitListTimeout            = 5 * time.Second
	maxScannedPaths           = 200000
	maxIncludedFiles          = 20000
	maxSkippedRecords         = 200000
	maxTotalSourceBytes int64 = 512 * 1024 * 1024
)

var defaultExcludes = []string{
	".git/**", ".hg/**", ".svn/**",
	".clyde/**",
	"node_modules/**", "vendor/**", "build/**", "dist/**", ".next/**",
	".turbo/**", "DerivedData/**", "*.pyc", "*.pyo", "*.o", "*.a",
	"*.so", "*.dylib", "*.dll", "*.exe", "*.zip", "*.tar", "*.gz",
	"*.7z", "*.dmg", "*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp",
	"*.pdf", "*.sqlite", "*.db", "*.lock",
	".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "*.crt",
	"credentials.json", "token.json", ".aws/**", ".config/gcloud/**",
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password)\b\s*[:=]\s*['"][^'"]{12,}['"]`),
	regexp.MustCompile(`\b[A-Za-z0-9_=-]{32,}\.[A-Za-z0-9_=-]{16,}\.[A-Za-z0-9_=-]{16,}\b`),
}

func ScanRepo(repo string, include, exclude []string, maxFileBytes int64) (ScanResult, error) {
	return ScanRepoWithOptions(repo, ScanOptions{Include: include, Exclude: exclude, MaxFileBytes: maxFileBytes})
}

func ScanRepoWithOptions(repo string, opts ScanOptions) (ScanResult, error) {
	if opts.MaxFileBytes <= 0 {
		return ScanResult{}, errf("maxFileBytes must be greater than 0")
	}
	if err := validateGlobPatterns(append(append([]string{}, opts.Include...), opts.Exclude...)); err != nil {
		return ScanResult{}, err
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return ScanResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return ScanResult{}, errf("repo path is not a directory: %s", abs)
	}

	result := ScanResult{Repo: abs}
	candidates, discovery, err := candidatePaths(abs, opts.AllowFilesystemFallback)
	if err != nil {
		return ScanResult{}, err
	}
	result.Discovery = discovery
	excludes := make([]string, 0, len(defaultExcludes)+len(opts.Exclude))
	excludes = append(excludes, defaultExcludes...)
	excludes = append(excludes, opts.Exclude...)
	for i, path := range candidates {
		if i >= maxScannedPaths {
			appendSkip(&result, "...", "scan stopped after "+itoa(maxScannedPaths)+" paths")
			break
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			continue
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			appendSkip(&result, filepath.ToSlash(rel), "outside repo")
			continue
		}
		rel = filepath.ToSlash(rel)
		if err := validateRepoRelPath(rel); err != nil {
			appendSkip(&result, rel, "invalid path string: "+err.Error())
			continue
		}
		if len(opts.Include) > 0 && !matchesAny(rel, opts.Include) {
			appendSkip(&result, rel, "not matched by include globs")
			continue
		}
		if matchesAny(rel, excludes) {
			appendSkip(&result, rel, "excluded by glob")
			continue
		}
		stat, err := os.Lstat(path)
		if err != nil {
			appendSkip(&result, rel, "stat failed: "+err.Error())
			continue
		}
		if stat.Mode()&os.ModeSymlink != 0 {
			appendSkip(&result, rel, "symbolic link")
			continue
		}
		if !stat.Mode().IsRegular() {
			appendSkip(&result, rel, "not a regular file")
			continue
		}
		if stat.Size() > opts.MaxFileBytes {
			appendSkip(&result, rel, "larger than "+itoa(opts.MaxFileBytes)+" bytes")
			continue
		}
		if len(result.Files) >= maxIncludedFiles {
			appendSkip(&result, rel, "included file limit reached")
			continue
		}
		data, openedStat, err := readScannedFile(path, stat, opts.MaxFileBytes)
		if err != nil {
			appendSkip(&result, rel, "read failed: "+err.Error())
			continue
		}
		if result.TotalBytes()+openedStat.Size() > maxTotalSourceBytes {
			appendSkip(&result, rel, "total source byte limit reached")
			continue
		}
		if looksBinary(data) {
			appendSkip(&result, rel, "binary file")
			continue
		}
		if !utf8.Valid(data) {
			appendSkip(&result, rel, "invalid UTF-8")
			continue
		}
		text := string(data)
		if looksSecret(text) {
			appendSkip(&result, rel, "possible secret material")
			continue
		}
		sum := sha256.Sum256(data)
		result.Files = append(result.Files, FileRecord{
			Path:   path,
			Rel:    rel,
			Size:   openedStat.Size(),
			SHA256: hex.EncodeToString(sum[:]),
			Text:   text,
		})
	}
	return result, nil
}

func appendSkip(result *ScanResult, path, reason string) {
	if len(result.Skips) >= maxSkippedRecords {
		return
	}
	result.Skips = append(result.Skips, SkipRecord{Path: path, Reason: reason})
}

func candidatePaths(repo string, allowFilesystemFallback bool) ([]string, ScanDiscovery, error) {
	if _, err := os.Stat(filepath.Join(repo, ".git")); err == nil {
		out, gitErr := gitListFiles(repo)
		discovery := ScanDiscovery{
			Method:            "git",
			GitExclusionsUsed: gitErr == nil,
			GitCommit:         gitCurrentCommit(repo),
			GitWorkingTree:    gitWorkingTreeState(repo),
		}
		if gitErr != nil {
			discovery.GitError = gitErr.Error()
			if !allowFilesystemFallback {
				return nil, discovery, errf("git-aware file discovery failed; refusing filesystem fallback in Git repository: %w", gitErr)
			}
			paths, walkErr := filesystemCandidatePaths(repo)
			if walkErr != nil {
				return nil, discovery, walkErr
			}
			discovery.Method = "filesystem-fallback"
			discovery.GitExclusionsUsed = false
			return paths, discovery, nil
		}
		paths := make([]string, 0, strings.Count(out, "\n"))
		for _, line := range strings.Split(out, "\n") {
			if line != "" {
				paths = append(paths, filepath.Join(repo, filepath.FromSlash(line)))
			}
		}
		return paths, discovery, nil
	}
	paths, err := filesystemCandidatePaths(repo)
	if err != nil {
		return nil, ScanDiscovery{Method: "filesystem", GitExclusionsUsed: false}, err
	}
	return paths, ScanDiscovery{Method: "filesystem", GitExclusionsUsed: false}, nil
}

func filesystemCandidatePaths(repo string) ([]string, error) {
	paths := make([]string, 0, min(maxScannedPaths, 4096))
	err := filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		paths = append(paths, path)
		if len(paths) >= maxScannedPaths {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func gitListFiles(repo string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitListTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "ls-files", "-co", "--exclude-standard")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, maxGitListBytes+1))
	if len(data) > maxGitListBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", errf("git ls-files output exceeded %d bytes", maxGitListBytes)
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil || readErr != nil || waitErr != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if readErr != nil {
			return "", readErr
		}
		return "", waitErr
	}
	return string(data), nil
}

func gitCurrentCommit(repo string) string {
	out, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gitWorkingTreeState(repo string) string {
	out, err := gitOutput(repo, "status", "--porcelain")
	if err != nil {
		return "unknown"
	}
	if strings.TrimSpace(out) == "" {
		return "clean"
	}
	return "dirty"
}

func gitOutput(repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitListTimeout)
	defer cancel()
	cmdArgs := append([]string{"-C", repo}, args...)
	out, err := exec.CommandContext(ctx, "git", cmdArgs...).Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func matchesAny(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		ok, err := filepath.Match(pattern, rel)
		if err == nil && ok {
			return true
		}
		if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(rel, strings.TrimSuffix(pattern, "**")) {
			return true
		}
	}
	return false
}

func validateGlobPatterns(patterns []string) error {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			return errf("glob pattern must not be empty")
		}
		check := pattern
		if strings.HasSuffix(pattern, "/**") {
			check = strings.TrimSuffix(pattern, "/**")
			if check == "" {
				return errf("invalid glob pattern %q", pattern)
			}
		}
		if _, err := filepath.Match(check, ""); err != nil {
			return errf("invalid glob pattern %q: %v", pattern, err)
		}
	}
	return nil
}

func looksBinary(data []byte) bool {
	if bytes.Contains(data, []byte{0}) {
		return true
	}
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if len(sample) == 0 {
		return false
	}
	control := 0
	for _, b := range sample {
		if b < 9 || (b > 13 && b < 32) {
			control++
		}
	}
	return float64(control)/float64(len(sample)) > 0.05
}

func looksSecret(text string) bool {
	for _, pattern := range secretPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
