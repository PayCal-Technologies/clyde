package clyde

import (
	"io"
	"os"
)

func readScannedFile(path string, expected os.FileInfo, maxFileBytes int64) ([]byte, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !opened.Mode().IsRegular() {
		return nil, nil, errf("not a regular file")
	}
	if !os.SameFile(expected, opened) {
		return nil, nil, errf("file changed during scan")
	}
	if opened.Size() > maxFileBytes {
		return nil, nil, errf("larger than %s bytes", itoa(maxFileBytes))
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > maxFileBytes {
		return nil, nil, errf("larger than %s bytes", itoa(maxFileBytes))
	}
	final, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(opened, final) || final.Size() != int64(len(data)) {
		return nil, nil, errf("file changed during scan")
	}
	return data, final, nil
}
