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
func MergeXray(primary, limited []byte, insertAfter string) ([]byte, error) {
	primaryProfiles, err := parseXrayRawArray(primary, "primary config")
	if err != nil {
		return nil, err
	}
	limitedProfiles, err := parseXrayRawArray(limited, "limited config")
	if err != nil {
		return nil, err
	}

	for i, raw := range limitedProfiles {
		raw, err = cleanJSONHostName(raw, "remarks")
		if err != nil {
			return nil, fmt.Errorf("clean limited profile[%d] name: %w", i, err)
		}
		limitedProfiles[i] = raw
	}

	merged := insertAfterHost(primaryProfiles, limitedProfiles, insertAfter, func(raw json.RawMessage) string {
		return jsonHostName(raw, "remarks")
	})
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
