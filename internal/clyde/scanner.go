package clyde

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var defaultExcludes = []string{
	".git/**", ".hg/**", ".svn/**",
	"node_modules/**", "vendor/**", "build/**", "dist/**", ".next/**",
	".turbo/**", "DerivedData/**", "*.pyc", "*.pyo", "*.o", "*.a",
	"*.so", "*.dylib", "*.dll", "*.exe", "*.zip", "*.tar", "*.gz",
	"*.7z", "*.dmg", "*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp",
	"*.pdf", "*.sqlite", "*.db", "*.lock",
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password)\b\s*[:=]\s*['"][^'"]{12,}['"]`),
	regexp.MustCompile(`\b[A-Za-z0-9_=-]{32,}\.[A-Za-z0-9_=-]{16,}\.[A-Za-z0-9_=-]{16,}\b`),
}

func ScanRepo(repo string, include, exclude []string, maxFileBytes int64) (ScanResult, error) {
	if maxFileBytes <= 0 {
		return ScanResult{}, errf("maxFileBytes must be greater than 0")
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
	candidates := candidatePaths(abs)
	excludes := append(append([]string{}, defaultExcludes...), exclude...)
	for _, path := range candidates {
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			continue
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			result.Skips = append(result.Skips, SkipRecord{Path: filepath.ToSlash(rel), Reason: "outside repo"})
			continue
		}
		rel = filepath.ToSlash(rel)
		if len(include) > 0 && !matchesAny(rel, include) {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "not matched by include globs"})
			continue
		}
		if matchesAny(rel, excludes) {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "excluded by glob"})
			continue
		}
		stat, err := os.Lstat(path)
		if err != nil {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "stat failed: " + err.Error()})
			continue
		}
		if stat.Mode()&os.ModeSymlink != 0 {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "symbolic link"})
			continue
		}
		if !stat.Mode().IsRegular() {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "not a regular file"})
			continue
		}
		if stat.Size() > maxFileBytes {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "larger than " + itoa(maxFileBytes) + " bytes"})
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "read failed: " + err.Error()})
			continue
		}
		if looksBinary(data) {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "binary file"})
			continue
		}
		text := string(data)
		if looksSecret(text) {
			result.Skips = append(result.Skips, SkipRecord{Path: rel, Reason: "possible secret material"})
			continue
		}
		sum := sha256.Sum256(data)
		result.Files = append(result.Files, FileRecord{
			Path:   path,
			Rel:    rel,
			Size:   stat.Size(),
			SHA256: hex.EncodeToString(sum[:]),
			Text:   text,
		})
	}
	return result, nil
}

func candidatePaths(repo string) []string {
	if _, err := os.Stat(filepath.Join(repo, ".git")); err == nil {
		cmd := exec.Command("git", "-C", repo, "ls-files", "-co", "--exclude-standard")
		if out, err := cmd.Output(); err == nil {
			var paths []string
			for _, line := range strings.Split(string(out), "\n") {
				if line != "" {
					paths = append(paths, filepath.Join(repo, filepath.FromSlash(line)))
				}
			}
			sort.Strings(paths)
			return paths
		}
	}
	var paths []string
	_ = filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
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
		return nil
	})
	sort.Strings(paths)
	return paths
}

func matchesAny(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, rel); ok {
			return true
		}
		if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(rel, strings.TrimSuffix(pattern, "**")) {
			return true
		}
	}
	return false
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
