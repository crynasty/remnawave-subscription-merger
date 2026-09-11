package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MergeXray merges two subscription response bodies for xray response type.
//
// Profile objects are carried as json.RawMessage so merge does not reorder their
// fields or convert their numeric values.
func MergeXray(primary, limited []byte) ([]byte, error) {
	primaryProfiles, err := parseXrayRawArray(primary, "primary config")
	if err != nil {
		return nil, err
	}
	limitedProfiles, err := parseXrayRawArray(limited, "limited config")
	if err != nil {
		return nil, err
	}

	merged := make([]json.RawMessage, 0, len(primaryProfiles)+len(limitedProfiles))
	for _, raw := range primaryProfiles {

		merged = append(merged, raw)
	}

	for _, raw := range limitedProfiles {
		merged = append(merged, raw)
	}

	return marshalRawJSONArray(merged), nil
}

func parseXrayRawArray(data []byte, label string) ([]json.RawMessage, error) {
	var profiles []json.RawMessage
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("%s array is empty", label)
	}
	return profiles, nil
}

func marshalRawJSONArray(profiles []json.RawMessage) []byte {
	var out bytes.Buffer
	out.WriteString("[\n")
	for i, raw := range profiles {
		if i > 0 {
			out.WriteString(",\n")
		}
		out.Write(bytes.TrimSpace(raw))
	}
	out.WriteString("\n]")
	return out.Bytes()
}
