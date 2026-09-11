package merge

import (
	"bytes"
	"encoding/base64"
	"fmt"
)

// MergeBase64 merges two subscription response bodies for base64 response type.
//
// A Remnawave Base64 subscription body is a single Base64-encoded list of
// connection URIs separated by newlines.
func MergeBase64(primary, limited []byte) ([]byte, error) {
	primaryPlain, err := decodeBase64Body(primary, "primary config")
	if err != nil {
		return nil, err
	}
	limitedPlain, err := decodeBase64Body(limited, "limited config")
	if err != nil {
		return nil, err
	}

	// Remove only boundary line endings so the two lists are separated by
	// exactly one newline. URI contents and duplicates are otherwise preserved.
	primaryPlain = bytes.Trim(primaryPlain, "\r\n")
	limitedPlain = bytes.Trim(limitedPlain, "\r\n")
	if len(primaryPlain) == 0 {
		return nil, fmt.Errorf("primary config: decoded base64 body is empty")
	}
	if len(limitedPlain) == 0 {
		return nil, fmt.Errorf("limited config: decoded base64 body is empty")
	}

	merged := make([]byte, 0, len(primaryPlain)+1+len(limitedPlain))
	merged = append(merged, primaryPlain...)
	merged = append(merged, '\n')
	merged = append(merged, limitedPlain...)

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(merged)))
	base64.StdEncoding.Encode(encoded, merged)
	return encoded, nil
}

func decodeBase64Body(data []byte, label string) ([]byte, error) {
	cleaned := stripBase64Whitespace(data)
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("%s: base64 body is empty", label)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(cleaned))
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(string(cleaned))
	}
	if err != nil {
		return nil, fmt.Errorf("%s: decode base64 body: %w", label, err)
	}
	if len(bytes.TrimSpace(decoded)) == 0 {
		return nil, fmt.Errorf("%s: decoded base64 body is empty", label)
	}
	return decoded, nil
}

func stripBase64Whitespace(data []byte) []byte {
	cleaned := make([]byte, 0, len(data))
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			cleaned = append(cleaned, b)
		}
	}
	return cleaned
}
