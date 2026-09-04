package clyde

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type receiptStatus struct {
	Path          string         `json:"path"`
	Destination   string         `json:"destination"`
	Backend       string         `json:"backend"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	ChunkStatuses map[string]int `json:"chunk_statuses"`
	DeletionPhase string         `json:"deletion_phase,omitempty"`
	DeletionCount int            `json:"deletion_count"`
	ResumeCommand string         `json:"resume_command,omitempty"`
	ResumeNote    string         `json:"resume_note,omitempty"`
}

func cmdReceipt(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printReceiptHelp(out)
		return flag.ErrHelp
	}
	if len(args) == 0 || args[0] != "status" {
		return errf("usage: clyde receipt status PATH [--json]")
	}
	fs := flag.NewFlagSet("receipt status", flag.ContinueOnError)
	fs.SetOutput(out)
	asJSON := fs.Bool("json", false, "print machine-readable receipt status")
	if err := fs.Parse(interspersedArgs(args[1:], map[string]bool{"json": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("receipt status requires PATH")
	}
	path := fs.Arg(0)
	receipt, err := loadSyncReceipt(path)
	if err != nil {
		return err
	}
	status := buildReceiptStatus(path, receipt)
	if *asJSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	printReceiptStatus(out, status)
	return nil
}

func buildReceiptStatus(path string, receipt SyncReceipt) receiptStatus {
	status := receiptStatus{
		Path:          path,
		Destination:   receipt.Destination,
		Backend:       receipt.Backend,
		CreatedAt:     receipt.CreatedAt,
		UpdatedAt:     receipt.UpdatedAt,
		ChunkStatuses: map[string]int{},
		DeletionPhase: receipt.DeletionPhase,
		DeletionCount: len(receipt.Deletions),
	}
	for _, chunk := range receipt.Chunks {
		status.ChunkStatuses[chunk.Status]++
	}
	status.ResumeCommand, status.ResumeNote = receiptResumeGuidance(path, receipt)
	return status
}

func printReceiptStatus(out io.Writer, status receiptStatus) {
	fmt.Fprintf(out, "Receipt: %s\n", status.Path)
	fmt.Fprintf(out, "Destination: %s\n", status.Destination)
	fmt.Fprintf(out, "Backend: %s\n", status.Backend)
	fmt.Fprintf(out, "Created: %s\n", status.CreatedAt)
	fmt.Fprintf(out, "Updated: %s\n", status.UpdatedAt)
	if len(status.ChunkStatuses) == 0 {
		fmt.Fprintln(out, "Chunks: none recorded")
	} else {
		keys := make([]string, 0, len(status.ChunkStatuses))
		for name := range status.ChunkStatuses {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, name := range keys {
			parts = append(parts, name+"="+itoa(int64(status.ChunkStatuses[name])))
		}
		fmt.Fprintf(out, "Chunks: %s\n", strings.Join(parts, ", "))
	}
	if status.DeletionPhase != "" {
		fmt.Fprintf(out, "Deletion: %s (%d sources)\n", status.DeletionPhase, status.DeletionCount)
	}
	if status.ResumeCommand != "" {
		fmt.Fprintln(out, "\nResume:")
		fmt.Fprintf(out, "  %s\n", status.ResumeCommand)
		return
	}
	if status.ResumeNote != "" {
		fmt.Fprintf(out, "\nResume: %s\n", status.ResumeNote)
	}
}

func receiptResumeGuidance(path string, receipt SyncReceipt) (string, string) {
	bundleDir := filepath.Dir(path)
	bundle, err := LoadBundle(bundleDir)
	if err != nil || receipt.BundleDigest == "" || bundle.Digest != receipt.BundleDigest {
		return "", "provide the original bundle directory or repository, then rerun sync with --resume and this receipt path"
	}
	args := []string{"clyde", "sync", "--bundle", bundleDir, "--approve-digest", receipt.BundleDigest}
	if strings.HasPrefix(receipt.Destination, "notebook_id:") {
		args = append(args, "--notebook-id", strings.TrimPrefix(receipt.Destination, "notebook_id:"))
	} else if strings.HasPrefix(receipt.Destination, "notebook_url:") {
		args = append(args, "--notebook-url", strings.TrimPrefix(receipt.Destination, "notebook_url:"))
	} else {
		return "", "receipt destination is invalid; inspect it before resuming"
	}
	args = append(args, "--approve-upload")
	if receipt.Backend == "nlm" {
		args = append(args, "--backend", "nlm")
	}
	if receipt.DeleteExistingSources {
		args = append(args, "--delete-existing-sources")
	}
	args = append(args, "--receipt", path, "--resume")
	return shellCommand(args), ""
}

func shellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, strconv.Quote(arg))
	}
	return strings.Join(quoted, " ")
}
