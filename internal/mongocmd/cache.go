package mongocmd

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

// DefaultCacheGB is the WiredTiger cache used when mongod accepts a
// fractional --wiredTigerCacheSizeGB. That form shipped in 3.4, with a
// minimum of 0.25 GB, so 0.5 is inside the supported range.
const DefaultCacheGB = "0.5"

// LegacyCacheGB is the integer cache used for mongod 3.0 through 3.3,
// which accept --wiredTigerCacheSizeGB but not a fraction.
const LegacyCacheGB = "1"

var versionPattern = regexp.MustCompile(`db version v(\d+)\.(\d+)`)

// CacheSizeGB chooses a --wiredTigerCacheSizeGB value for the text of
// `mongod --version`. ok is false when the version is missing, older than
// 3.0, or not a WiredTiger-era server we can identify.
func CacheSizeGB(versionText string) (size string, ok bool) {
	major, minor, parsed := parseRelease(versionText)
	if !parsed {
		return "", false
	}
	if major > 3 || (major == 3 && minor >= 4) {
		return DefaultCacheGB, true
	}
	if major == 3 {
		return LegacyCacheGB, true
	}
	return "", false
}

func parseRelease(versionText string) (major, minor int, ok bool) {
	match := versionPattern.FindStringSubmatch(versionText)
	if match == nil {
		return 0, 0, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(match[2])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// DetectCacheSize runs `mongod --version` and returns the cache size that
// binary accepts. The error tells the caller to omit the flag.
func DetectCacheSize(ctx context.Context) (string, error) {
	path, err := exec.LookPath("mongod")
	if err != nil {
		return "", fmt.Errorf("mongod is not on PATH")
	}
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("mongod --version: %w", err)
	}
	size, ok := CacheSizeGB(string(out))
	if !ok {
		return "", fmt.Errorf("unrecognized mongod version %q", firstLine(string(out)))
	}
	return size, nil
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
