// Package mihomo drives an external mihomo (Clash Meta) core process to run
// real latency tests for every supported protocol via its RESTful API.
package mihomo

import (
	"os"
	"path/filepath"
	"runtime"
)

// candidateCorePaths returns likely locations of a mihomo core binary,
// prioritising the one bundled with Clash Verge.
func candidateCorePaths() []string {
	var paths []string
	if runtime.GOOS == "windows" {
		programFiles := []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			`C:\Program Files`,
		}
		names := []string{"verge-mihomo.exe", "verge-mihomo-alpha.exe", "mihomo.exe", "clash-meta.exe"}
		for _, base := range programFiles {
			if base == "" {
				continue
			}
			for _, n := range names {
				paths = append(paths, filepath.Join(base, "Clash Verge", n))
			}
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "Programs", "clash-verge", "verge-mihomo.exe"))
		}
	} else {
		names := []string{"verge-mihomo", "mihomo", "clash-meta"}
		dirs := []string{"/usr/bin", "/usr/local/bin", "/opt/clash-verge"}
		for _, d := range dirs {
			for _, n := range names {
				paths = append(paths, filepath.Join(d, n))
			}
		}
	}
	return paths
}

// FindCore locates a usable mihomo core. An explicit path wins if it exists.
func FindCore(explicit string) (string, bool) {
	if explicit != "" {
		if fileExists(explicit) {
			return explicit, true
		}
		return "", false
	}
	for _, p := range candidateCorePaths() {
		if fileExists(p) {
			return p, true
		}
	}
	return "", false
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
