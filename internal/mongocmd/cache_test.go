package mongocmd

import (
	"strings"
	"testing"
)

func TestForkSupported(t *testing.T) {
	tests := []struct {
		goos string
		text string
		want bool
	}{
		{goos: "linux", text: "db version v9.0.4\n", want: true},
		{goos: "linux", text: "not a version\n", want: true},
		{goos: "darwin", text: "db version v8.0.12\n", want: true},
		{goos: "darwin", text: "db version v8.1.1\n", want: true},
		{goos: "darwin", text: "db version v7.0.14\n", want: true},
		{goos: "darwin", text: "db version v8.2.0\n", want: false},
		{goos: "darwin", text: "db version v8.2.1\n", want: false},
		{goos: "darwin", text: "db version v9.0.4\n", want: false},
		{goos: "darwin", text: "not a version\n", want: false},
		{goos: "windows", text: "db version v4.4.29\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.goos+" "+tt.text, func(t *testing.T) {
			if got := ForkSupported(tt.goos, tt.text); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheOmitReason(t *testing.T) {
	old := CacheOmitReason("db version v2.6.12\n")
	if old != "mongod db version v2.6.12 is older than 3.0" {
		t.Fatalf("old = %q", old)
	}
	if got := CacheOmitReason("not a version\n"); !strings.Contains(got, "unrecognized mongod version") {
		t.Fatalf("unknown = %q", got)
	}
	if got := CacheOmitReason("db version v4.4.29\n"); got != "" {
		t.Fatalf("supported = %q", got)
	}
}

func TestCacheSizeGB(t *testing.T) {
	tests := []struct {
		name string
		text string
		size string
		ok   bool
	}{
		{name: "3.2 integer only", text: "db version v3.2.22\n", size: "1", ok: true},
		{name: "3.0", text: "db version v3.0.15\n", size: "1", ok: true},
		{name: "3.3 still integer", text: "db version v3.3.15\n", size: "1", ok: true},
		{name: "3.4 float", text: "db version v3.4.0\n", size: "0.5", ok: true},
		{name: "3.4 rc", text: "db version v3.4.24-rc0\n", size: "0.5", ok: true},
		{name: "4.4", text: "db version v4.4.29\nBuild Info: {\"version\": \"4.4.29\"}\n", size: "0.5", ok: true},
		{name: "7.0", text: "db version v7.0.14\n", size: "0.5", ok: true},
		{name: "8.0", text: "db version v8.0.12\n", size: "0.5", ok: true},
		{name: "pre wiredtiger flag", text: "db version v2.6.12\n", ok: false},
		{name: "garbage", text: "not a version\n", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, ok := CacheSizeGB(tt.text)
			if ok != tt.ok || size != tt.size {
				t.Fatalf("got %q %v, want %q %v", size, ok, tt.size, tt.ok)
			}
		})
	}
}
