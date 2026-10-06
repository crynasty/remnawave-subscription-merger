package merge

import (
	"bytes"
	"encoding/json"
	"strings"
)

// cleanLimitedHostName removes the exact unit text emitted by template variables,
// including its leading space. Call only for hosts from the limited config.
func cleanLimitedHostName(name string) string {
	return strings.ReplaceAll(name, " GiB", "")
}

// cleanJSONHostName edits only a top-level name field, preserving the raw JSON
// representation of all other fields (including field order and large numbers).
func cleanJSONHostName(raw json.RawMessage, field string) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return raw, nil
	}

	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if key != field {
			continue
		}
		var name string
		if err := json.Unmarshal(value, &name); err != nil {
			return raw, nil
		}
		cleaned := cleanLimitedHostName(name)
		if cleaned == name {
			return raw, nil
		}
		replacement, err := json.Marshal(cleaned)
		if err != nil {
			return nil, err
		}
		end := int(decoder.InputOffset())
		start := end - len(value)
		out := make(json.RawMessage, 0, len(raw)-len(value)+len(replacement))
		out = append(out, raw[:start]...)
		out = append(out, replacement...)
		out = append(out, raw[end:]...)
		return out, nil
	}
	return raw, nil
}
