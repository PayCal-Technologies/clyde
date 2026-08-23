package clyde

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxSyncReceiptBytes = 16 * 1024 * 1024

func preparePrivateDir(path string) error {
	if err := refuseSymlinkPathComponents(path, true); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if err := refuseSymlinkPathComponents(path, true); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errf("refusing symlink directory: %s", path)
	}
	if !info.IsDir() {
		return errf("path must be a directory: %s", path)
	}
	return os.Chmod(path, 0o700)
}

func refuseUnsafeBundleTarget(path string, force bool) error {
	if err := refuseSymlinkPathComponents(path, false); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errf("refusing to overwrite symlink bundle file: %s", path)
	}
	if !force {
		return errf("refusing to overwrite existing bundle file without --force: %s", path)
	}
	if !info.Mode().IsRegular() {
		return errf("refusing to overwrite non-regular bundle file: %s", path)
	}
	return nil
}

func writePrivateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := refuseSymlinkPathComponents(path, false); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func readRegularFileLimited(path string, maxBytes int64) ([]byte, error) {
	file, expected, err := openRegularFileLimited(path, maxBytes)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errf("file is too large; maximum is %d bytes", maxBytes)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(expected, final) || final.Size() != int64(len(data)) {
		return nil, errf("file changed during read: %s", path)
	}
	return data, nil
}

func openRegularFileLimited(path string, maxBytes int64) (*os.File, os.FileInfo, error) {
	if err := refuseSymlinkPathComponents(path, false); err != nil {
		return nil, nil, err
	}
	expected, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if expected.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errf("refusing to read symlink file: %s", path)
	}
	if !expected.Mode().IsRegular() {
		return nil, nil, errf("refusing to read non-regular file: %s", path)
	}
	if expected.Size() > maxBytes {
		return nil, nil, errf("file is too large; maximum is %d bytes", maxBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !opened.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, errf("refusing to read non-regular file: %s", path)
	}
	if !os.SameFile(expected, opened) {
		_ = file.Close()
		return nil, nil, errf("file changed before read: %s", path)
	}
	return file, opened, nil
}

func refuseSymlinkPathComponents(path string, includeLeaf bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	checkPath := abs
	if !includeLeaf {
		checkPath = filepath.Dir(abs)
	}
	volume := filepath.VolumeName(checkPath)
	rest := strings.TrimPrefix(checkPath, volume)
	current := volume
	if strings.HasPrefix(rest, string(filepath.Separator)) {
		current += string(filepath.Separator)
		rest = strings.TrimLeft(rest, string(filepath.Separator))
	}
	depth := 0
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		depth++
		if current == "" || current == string(filepath.Separator) || strings.HasSuffix(current, string(filepath.Separator)) {
			current += part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if depth == 1 && filepath.IsAbs(abs) {
				continue
			}
			return errf("refusing symlink path component: %s", current)
		}
		if !info.IsDir() {
			return errf("refusing non-directory path component: %s", current)
		}
	}
	return nil
}
