package merge

import (
	"encoding/json"
	"fmt"
)

// MergeSingBox merges two subscription response bodies for sing-box response type.
func MergeSingBox(primaryConfig, limitedConfig []byte) ([]byte, error) {
	// Parse both configurations
	var primary map[string]any
	if err := json.Unmarshal(primaryConfig, &primary); err != nil {
		return nil, fmt.Errorf("parse primary config: %w", err)
	}

	var limited map[string]any
	if err := json.Unmarshal(limitedConfig, &limited); err != nil {
		return nil, fmt.Errorf("parse limited config: %w", err)
	}

	// Extract outbounds from the limited config
	limitedOutbounds, err := getOutboundsSlice(limited, "limited config")
	if err != nil {
		return nil, err
	}

	// Extract proxies from the limited config outbounds
	limitedProxies := filterSingBoxProxies(limitedOutbounds)
	if len(limitedProxies) == 0 {
		return nil, fmt.Errorf("limited config has no outbound proxies (with 'server' field)")
	}

	// Collect proxy tags from the outbounds
	tags, err := extractTags(limitedProxies)
	if err != nil {
		return nil, fmt.Errorf("extract tags: %w", err)
	}

	// Find the first selector in the primary config and append tags to its outbounds
	primaryOutbounds, err := getOutboundsSlice(primary, "primary config")
	if err != nil {
		return nil, err
	}

	selectorIdx := findFirstSelector(primaryOutbounds)
	if selectorIdx < 0 {
		return nil, fmt.Errorf("primary config has no outbound with type 'selector'")
	}

	selectorMap, ok := primaryOutbounds[selectorIdx].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("selector outbound is not a map")
	}
	if err := appendToSelectorOutbounds(selectorMap, tags); err != nil {
		return nil, fmt.Errorf("update selector outbounds: %w", err)
	}

	// Append proxy outbounds to the primary config array
	for _, o := range limitedProxies {
		primaryOutbounds = append(primaryOutbounds, o)
	}
	primary["outbounds"] = primaryOutbounds
	if err := validateSingBoxSelectorReferences(primaryOutbounds, selectorMap); err != nil {
		return nil, fmt.Errorf("validate selector: %w", err)
	}

	//  6. Serialize the result
	out, err := json.MarshalIndent(primary, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return out, nil
}

// isProxy reports whether an outbound is a real proxy host. A real proxy has a
// "server" field containing the server address. Service types such as selector,
// direct, block, dns, and urltest do not have a "server" field.
func isProxy(o map[string]any) bool {
	server, ok := o["server"]
	if !ok {
		return false
	}
	s, ok := server.(string)
	return ok && s != ""
}

// filterSingBoxProxies returns only real proxies from an outbounds slice.
func filterSingBoxProxies(outbounds []any) []map[string]any {
	var result []map[string]any
	for _, item := range outbounds {
		o, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if isProxy(o) {
			result = append(result, o)
		}
	}
	return result
}

// extractTags returns the "tag" field from each outbound.
func extractTags(outbounds []map[string]any) ([]string, error) {
	tags := make([]string, 0, len(outbounds))
	for i, o := range outbounds {
		tagVal, ok := o["tag"]
		if !ok {
			return nil, fmt.Errorf("outbound[%d] has no 'tag' field", i)
		}
		tag, ok := tagVal.(string)
		if !ok || tag == "" {
			return nil, fmt.Errorf("outbound[%d] 'tag' is not a non-empty string", i)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// findFirstSelector returns the index of the first outbound with "type": "selector".
// It returns -1 if none is found.
func findFirstSelector(outbounds []any) int {
	for i, item := range outbounds {
		o, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := o["type"].(string); t == "selector" {
			return i
		}
	}
	return -1
}

// appendToSelectorOutbounds appends tags to the selector's "outbounds" array.
// Existing values are preserved, and new values are appended.
func appendToSelectorOutbounds(selector map[string]any, tags []string) error {
	raw, ok := selector["outbounds"]
	if !ok {
		// Create an empty array when the selector has no outbounds
		raw = []any{}
	}

	existing, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("selector 'outbounds' is not an array")
	}

	for _, tag := range tags {
		existing = append(existing, tag)
	}

	selector["outbounds"] = existing
	return nil
}

// getOutboundsSlice extracts the "outbounds" field from a config as []any.
func getOutboundsSlice(cfg map[string]any, label string) ([]any, error) {
	raw, ok := cfg["outbounds"]
	if !ok {
		return nil, fmt.Errorf("%s config has no 'outbounds' key", label)
	}
	slice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s 'outbounds' is not an array", label)
	}
	return slice, nil
}

func validateSingBoxSelectorReferences(outbounds []any, selector map[string]any) error {
	available := make(map[string]struct{}, len(outbounds))
	realTags := make(map[string]struct{}, len(outbounds))

	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}

		tag, ok := outbound["tag"].(string)
		if !ok || tag == "" {
			continue
		}

		available[tag] = struct{}{}
		if isProxy(outbound) {
			realTags[tag] = struct{}{}
		}
	}

	rawReferences, ok := selector["outbounds"].([]any)
	if !ok {
		return fmt.Errorf("selector 'outbounds' is not an array")
	}

	referenced := make(map[string]struct{}, len(rawReferences))
	for _, raw := range rawReferences {
		tag, ok := raw.(string)
		if !ok || tag == "" {
			return fmt.Errorf("selector contains invalid outbound reference")
		}
		if _, ok := available[tag]; !ok {
			return fmt.Errorf("selector references unknown outbound %q", tag)
		}
		referenced[tag] = struct{}{}
	}

	for tag := range realTags {
		if _, ok := referenced[tag]; !ok {
			return fmt.Errorf("selector does not reference proxy outbound %q", tag)
		}
	}

	return nil
}
