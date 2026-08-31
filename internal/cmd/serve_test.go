package cmd

import "testing"

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		value string
		want  int64
		ok    bool
	}{
		{value: "64MiB", want: 64 << 20, ok: true},
		{value: "2 kib", want: 2 << 10, ok: true},
		{value: "1GiB", want: 1 << 30, ok: true},
		{value: "64MB"},
		{value: "0MiB"},
		{value: "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseByteSize(tt.value)
			if (err == nil) != tt.ok {
				t.Fatalf("parseByteSize(%q) error = %v, want success = %t", tt.value, err, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("parseByteSize(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsLoopbackAddress(t *testing.T) {
	for _, tt := range []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8080", want: true},
		{addr: "[::1]:8080", want: true},
		{addr: "localhost:8080", want: true},
		{addr: "0.0.0.0:8080", want: false},
		{addr: "example.com:8080", want: false},
		{addr: "invalid", want: false},
	} {
		t.Run(tt.addr, func(t *testing.T) {
			if got := isLoopbackAddress(tt.addr); got != tt.want {
				t.Fatalf("isLoopbackAddress(%q) = %t, want %t", tt.addr, got, tt.want)
			}
		})
	}
}
