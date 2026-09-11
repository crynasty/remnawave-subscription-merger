package merge

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func encodeBase64(value string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(value)))
}

func decodeMergedBase64(t *testing.T, value []byte) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(string(value))
	if err != nil {
		t.Fatalf("decode merged body: %v", err)
	}
	return string(decoded)
}

func TestMergeBase64Bodies(t *testing.T) {
	primaryConfig := encodeBase64("trojan://primary.example:443#primary\nvless://primary.example:443#primary-2")
	limitedConfig := encodeBase64("vless://limited.example:443#Limited\ntrojan://limited.example:443#Limited-2")

	out, err := MergeBase64(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeBase64: %v", err)
	}

	want := "trojan://primary.example:443#primary\n" +
		"vless://primary.example:443#primary-2\n" +
		"vless://limited.example:443#Limited\n" +
		"trojan://limited.example:443#Limited-2"
	if got := decodeMergedBase64(t, out); got != want {
		t.Fatalf("decoded merge mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestMergeBase64SupportsWhitespaceAndMissingPadding(t *testing.T) {
	primaryConfig := []byte("  " + base64.RawStdEncoding.EncodeToString([]byte("trojan://a")) + "\n")
	limitedEncoded := base64.StdEncoding.EncodeToString([]byte("vless://b"))
	limitedConfig := []byte(limitedEncoded[:4] + " \r\n\t" + limitedEncoded[4:])

	out, err := MergeBase64(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeBase64: %v", err)
	}
	if got, want := decodeMergedBase64(t, out), "trojan://a\nvless://b"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestMergeBase64Fixture(t *testing.T) {
	primaryConfig, err := os.ReadFile(filepath.Join("testdata", "base64-primary.txt"))
	if err != nil {
		t.Fatal(err)
	}
	limitedConfig, err := os.ReadFile(filepath.Join("testdata", "base64-limited.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := MergeBase64(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeBase64: %v", err)
	}
	if got := decodeMergedBase64(t, out); got != "trojan://primary.example:443#primary\nvless://limited.example:443#Limited" {
		t.Fatalf("unexpected decoded fixture merge: %q", got)
	}
}

func TestMergeBase64Errors(t *testing.T) {
	tests := []struct {
		name          string
		primaryConfig []byte
		limitedConfig []byte
	}{
		{name: "empty primary config", primaryConfig: nil, limitedConfig: encodeBase64("vless://b")},
		{name: "invalid limited config", primaryConfig: encodeBase64("vless://a"), limitedConfig: []byte("not-base64!")},
		{name: "empty decoded limited config", primaryConfig: encodeBase64("vless://a"), limitedConfig: encodeBase64("")},
		{name: "whitespace decoded limited config", primaryConfig: encodeBase64("vless://a"), limitedConfig: encodeBase64(" \r\n\t")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := MergeBase64(tt.primaryConfig, tt.limitedConfig); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
