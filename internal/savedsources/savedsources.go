// Package savedsources persists the subscription URLs a user entered in the GUI
// so the next launch can pre-fill them automatically. URLs are stored verbatim
// (one per line); date adjustment is applied at fetch time, not on disk, so the
// saved link keeps its original shape and stays human-readable.
package savedsources

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath is where the GUI stores the most recently used subscription URLs.
const DefaultPath = "output/saved_sources.txt"

// Load reads saved URLs from path. A missing file is not an error: it returns an
// empty slice. Blank lines and lines starting with '#' are ignored, and
// duplicates are dropped while preserving order.
func Load(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return out, sc.Err()
}

// Save writes urls to path (creating parent directories), one per line,
// deduplicated and order-preserving. An empty list clears the file.
func Save(path string, urls []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	seen := map[string]bool{}
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		buf.WriteString(u)
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
