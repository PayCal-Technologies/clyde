package clyde

type FileRecord struct {
	Path   string
	Rel    string
	Size   int64
	SHA256 string
	Text   string
}

type SkipRecord struct {
	Path   string
	Reason string
}

type ScanResult struct {
	Repo  string
	Files []FileRecord
	Skips []SkipRecord
}

func (r ScanResult) TotalBytes() int64 {
	var total int64
	for _, file := range r.Files {
		total += file.Size
	}
	return total
}

type ChunkRecord struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	ChunkIndex  int    `json:"chunk_index"`
	ChunkTotal  int    `json:"chunk_total"`
	ChunkSHA256 string `json:"chunk_sha256,omitempty"`
	Text        string `json:"text"`
}
