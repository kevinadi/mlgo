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

// ForkSupported reports whether goos can start this mongod with --fork.
//
// Windows never uses --fork. MongoDB 8.2 and newer reject it on macOS:
// fork+exec via --fork is incompatible with the system, and mongod exits
// before it writes a log. An unreadable version on darwin is treated as
// unsupported, because passing --fork fails on 8.2+ and a new session still
// works on older servers. Linux and other Unix systems keep --fork.
func ForkSupported(goos, versionText string) bool {
	if goos == "windows" {
		return false
	}
	if goos != "darwin" {
		return true
	}
	major, minor, ok := parseRelease(versionText)
	if !ok {
		return false
	}
	if major > 8 || (major == 8 && minor >= 2) {
		return false
	}
	return true
}

// VersionText runs `mongod --version` and returns its stdout.
func VersionText(ctx context.Context) (string, error) {
	path, err := exec.LookPath("mongod")
	if err != nil {
		return "", fmt.Errorf("mongod is not on PATH")
	}
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("mongod --version: %w", err)
	}
	return string(out), nil
}

// DetectCacheSize runs `mongod --version` and returns the cache size that
// binary accepts. The error tells the caller to omit the flag.
func DetectCacheSize(ctx context.Context) (string, error) {
	text, err := VersionText(ctx)
	if err != nil {
		return "", err
	}
	size, ok := CacheSizeGB(text)
	if !ok {
		return "", fmt.Errorf("%s", CacheOmitReason(text))
	}
	return size, nil
}

// CacheOmitReason explains why --wiredTigerCacheSizeGB is left unset.
// A parsed release older than 3.0 is not an unrecognized version.
func CacheOmitReason(versionText string) string {
	if _, ok := CacheSizeGB(versionText); ok {
		return ""
	}
	if _, _, ok := parseRelease(versionText); ok {
		return "mongod " + firstLine(versionText) + " is older than 3.0"
	}
	return fmt.Sprintf("unrecognized mongod version %q", firstLine(versionText))
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
