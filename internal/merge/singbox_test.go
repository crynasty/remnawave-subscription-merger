package merge

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode object: %v", err)
	}
	return value
}

func singBoxTags(t *testing.T, data []byte) ([]string, []string) {
	t.Helper()
	cfg := decodeObject(t, data)
	outbounds, err := getOutboundsSlice(cfg, "result")
	if err != nil {
		t.Fatal(err)
	}
	var tags, selector []string
	for _, item := range outbounds {
		outbound := item.(map[string]any)
		tag := outbound["tag"].(string)
		tags = append(tags, tag)
		if outbound["type"] == "selector" {
			for _, raw := range outbound["outbounds"].([]any) {
				selector = append(selector, raw.(string))
			}
		}
	}
	return tags, selector
}

func TestMergeSingBoxPreservesPrimaryAndAddsAllLimitedServers(t *testing.T) {
	primaryConfig := loadFile(t, "singbox.json")
	limitedConfig := []byte(`{
  "outbounds": [
    {"type":"vless","tag":"Limited A","server":"a.example","server_port":443},
    {"type":"trojan","tag":"Limited B","server":"b.example","server_port":443},
    {"type":"direct","tag":"limited-direct"}
  ]
}`)

	out, err := MergeSingBox(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeSingBox: %v", err)
	}
	tags, selector := singBoxTags(t, out)
	for _, want := range []string{"Proxy", "direct", "block", "Limited A", "Limited B"} {
		if !contains(tags, want) {
			t.Errorf("missing outbound tag %q in %v", want, tags)
		}
	}
	for _, want := range []string{"Proxy", "Limited A", "Limited B"} {
		if count(selector, want) != 1 {
			t.Errorf("selector should contain %q once: %v", want, selector)
		}
	}
	if contains(tags, "limited-direct") {
		t.Errorf("limited config service outbound was appended: %v", tags)
	}

	result := decodeObject(t, out)
	for _, key := range []string{"log", "dns", "route", "inbounds", "experimental"} {
		if _, ok := result[key]; !ok {
			t.Errorf("primary config field %q was lost", key)
		}
	}
}

func TestMergeSingBoxRejectsUnknownSelectorReference(t *testing.T) {
	primaryConfig := []byte(`{"outbounds":[
  {"type":"selector","tag":"Select","outbounds":["Missing"]},
  {"type":"direct","tag":"direct"}
]}`)
	limitedConfig := []byte(`{"outbounds":[{"type":"vless","tag":"Limited","server":"a.example"}]}`)
	if _, err := MergeSingBox(primaryConfig, limitedConfig); err == nil || !strings.Contains(err.Error(), "unknown outbound") {
		t.Fatalf("expected selector reference error, got %v", err)
	}
}

func TestMergeSingBoxRejectsRealOutboundMissingFromSelector(t *testing.T) {
	primaryConfig := []byte(`{"outbounds":[
  {"type":"selector","tag":"Select","outbounds":["Visible"]},
  {"type":"vless","tag":"Visible","server":"visible.example"},
  {"type":"vless","tag":"Hidden","server":"hidden.example"}
]}`)
	limitedConfig := []byte(`{"outbounds":[{"type":"vless","tag":"Limited","server":"limited.example"}]}`)
	if _, err := MergeSingBox(primaryConfig, limitedConfig); err == nil || !strings.Contains(err.Error(), "does not reference") {
		t.Fatalf("expected missing real outbound reference error, got %v", err)
	}
}

func contains(values []string, want string) bool { return count(values, want) > 0 }

func count(values []string, want string) int {
	result := 0
	for _, value := range values {
		if value == want {
			result++
		}
	}
	return result
}
