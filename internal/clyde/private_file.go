package clyde

import (
	"os"
	"path/filepath"
)

func refuseUnsafeBundleTarget(path string, force bool) error {
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
