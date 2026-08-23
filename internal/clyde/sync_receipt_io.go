package clyde

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func loadSyncReceipt(path string) (SyncReceipt, error) {
	data, err := readRegularFileLimited(path, maxSyncReceiptBytes)
	if err != nil {
		return SyncReceipt{}, err
	}
	var receipt SyncReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return SyncReceipt{}, err
	}
	if receipt.Schema != "clyde.sync_receipt.v1" {
		return SyncReceipt{}, errf("unsupported sync receipt schema: %s", receipt.Schema)
	}
	return receipt, nil
}

func saveSyncReceipt(path string, receipt SyncReceipt) error {
	receipt.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := preparePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	return writePrivateAtomic(path, append(data, '\n'))
}

func prepareSyncReceipt(opts SyncOptions) (*SyncReceipt, error) {
	if opts.ReceiptPath == "" {
		return nil, nil
	}
	if opts.Backend == "" {
		opts.Backend = "mcp"
	}
	if opts.Resume {
		receipt, err := loadSyncReceipt(opts.ReceiptPath)
		if err == nil {
			if !receipt.canResume(opts) {
				return nil, errf("sync receipt does not match requested transfer")
			}
			return &receipt, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errf("--resume requires an existing sync receipt: %s", opts.ReceiptPath)
	} else if _, err := os.Lstat(opts.ReceiptPath); err == nil {
		return nil, errf("sync receipt already exists; use --resume or choose a new receipt path: %s", opts.ReceiptPath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	receipt := newSyncReceipt(opts)
	if err := saveSyncReceipt(opts.ReceiptPath, receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func recordSyncReceiptChunk(opts SyncOptions, receipt *SyncReceipt, chunk ChunkRecord, title, sourceID, status string, err error) error {
	if receipt == nil || opts.ReceiptPath == "" {
		return nil
	}
	receipt.recordChunk(chunk, title, sourceID, status, err)
	return saveSyncReceipt(opts.ReceiptPath, *receipt)
}

func recordUploadedSyncReceiptChunk(opts SyncOptions, receipt *SyncReceipt, chunk ChunkRecord, title, sourceID string) error {
	err := recordSyncReceiptChunk(opts, receipt, chunk, title, sourceID, "uploaded", nil)
	if err == nil || receipt == nil || opts.ReceiptPath == "" {
		return err
	}
	ambiguousErr := errf("remote upload succeeded but local receipt save failed: %v", err)
	if saveErr := recordSyncReceiptChunk(opts, receipt, chunk, title, sourceID, "ambiguous", ambiguousErr); saveErr != nil {
		return errf("remote upload succeeded but local receipt is ambiguous and could not be saved: uploaded_save=%v ambiguous_save=%v", err, saveErr)
	}
	return ambiguousErr
}
