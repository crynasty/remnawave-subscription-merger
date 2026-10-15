package merge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func xrayConfig(names []string) []byte {
	profiles := make([]map[string]any, 0, len(names))
	for _, name := range names {
		profiles = append(profiles, map[string]any{"remarks": name})
	}
	data, _ := json.Marshal(profiles)
	return data
}

func singBoxConfig(names []string) []byte {
	outbounds := []any{
		map[string]any{"type": "selector", "tag": "Select", "outbounds": names},
		map[string]any{"type": "direct", "tag": "direct"},
	}
	for _, name := range names {
		outbounds = append(outbounds, map[string]any{"type": "vless", "tag": name, "server": "example.com"})
	}
	data, _ := json.Marshal(map[string]any{"outbounds": outbounds})
	return data
}

func clashConfig(names []string) []byte {
	proxies := make([]any, 0, len(names))
	for _, name := range names {
		proxies = append(proxies, map[string]any{"name": name, "type": "vless", "server": "example.com"})
	}
	data, _ := yaml.Marshal(map[string]any{
		"proxies":      proxies,
		"proxy-groups": []any{map[string]any{"name": "Select", "type": "select", "proxies": names}},
	})
	return data
}

func base64Config(names []string) []byte {
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, "vless://id@example.com:443#"+(&url.URL{Fragment: name}).EscapedFragment())
	}
	return encodeBase64(strings.Join(lines, "\n"))
}

func TestMergeInsertsLimitedAfterHost(t *testing.T) {
	type formatCase struct {
		name  string
		build func([]string) []byte
		merge func([]byte, []byte, string) ([]byte, error)
		read  func(*testing.T, []byte) ([]string, []string)
	}
	formats := []formatCase{
		{"Xray", xrayConfig, MergeXray, func(t *testing.T, data []byte) ([]string, []string) {
			var profiles []struct{ Remarks string }
			if err := json.Unmarshal(data, &profiles); err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(profiles))
			for _, profile := range profiles {
				names = append(names, profile.Remarks)
			}
			return names, nil
		}},
		{"Base64", base64Config, MergeBase64, func(t *testing.T, data []byte) ([]string, []string) {
			var names []string
			for _, line := range strings.Split(decodeMergedBase64(t, data), "\n") {
				uri, err := url.Parse(line)
				if err != nil {
					t.Fatal(err)
				}
				names = append(names, uri.Fragment)
			}
			return names, nil
		}},
		{"SingBox", singBoxConfig, MergeSingBox, func(t *testing.T, data []byte) ([]string, []string) {
			tags, selector := singBoxTags(t, data)
			return tags[2:], selector
		}},
	}
	for _, name := range []string{"Clash", "Mihomo", "Stash"} {
		formats = append(formats, formatCase{name, clashConfig, MergeClashLike, func(t *testing.T, data []byte) ([]string, []string) {
			return getProxiesNames(t, data), getFirstGroupProxies(t, data)
		}})
	}
	standard := []string{"PRIMARY-1", "PRIMARY-2", "PRIMARY-3"}
	tests := []struct {
		name    string
		primary []string
		anchor  string
		index   int
	}{
		{"first", standard, "PRIMARY-1", 1},
		{"middle", standard, "PRIMARY-2", 2},
		{"last", standard, "PRIMARY-3", 3},
		{"disabled", standard, "", 3},
		{"missing", standard, "MISSING", 3},
		{"case sensitive", standard, "primary-1", 3},
		{"not a substring", standard, "PRIMARY", 3},
		{"no trimming", standard, " PRIMARY-1 ", 3},
		{"first repeated name", []string{"PRIMARY-1", "PRIMARY-2", "PRIMARY-1"}, "PRIMARY-1", 1},
		{"unicode and spaces", []string{"PRIMARY-1", " 🇩🇪 Германия + 1 ", "PRIMARY-3"}, " 🇩🇪 Германия + 1 ", 2},
		{"primary name preserved", []string{"PRIMARY-1 GiB", "PRIMARY-2", "PRIMARY-3"}, "PRIMARY-1 GiB", 1},
		{"primary name not cleaned", []string{"PRIMARY-1 GiB", "PRIMARY-2", "PRIMARY-3"}, "PRIMARY-1", 3},
	}
	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					data, err := format.merge(format.build(tc.primary), format.build([]string{"LIMITED-1 GiB", "LIMITED-2 GiB"}), tc.anchor)
					if err != nil {
						t.Fatal(err)
					}
					want := append([]string{}, tc.primary[:tc.index]...)
					want = append(want, "LIMITED-1", "LIMITED-2")
					want = append(want, tc.primary[tc.index:]...)
					names, references := format.read(t, data)
					assertStringSliceEqual(t, "definitions", names, want)
					if references != nil {
						assertStringSliceEqual(t, "references", references, want)
					}
				})
			}
		})
	}
}

func TestMergeBase64InsertionPreservesURIsAndLineEndings(t *testing.T) {
	for _, lineEnding := range []struct{ name, separator string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		for _, variant := range []struct {
			name     string
			encoding *base64.Encoding
		}{{"padded", base64.StdEncoding}, {"unpadded", base64.RawStdEncoding}} {
			t.Run(lineEnding.name+"/"+variant.name, func(t *testing.T) {
				separator, encoding := lineEnding.separator, variant.encoding
				anchorURI := "vless://id@example:443?value=keep%20GiB#%F0%9F%87%A9%F0%9F%87%AA%20A+1"
				primary := anchorURI + separator + "trojan://secret@example:443#PRIMARY-2" + separator + "ss://secret@example:443#PRIMARY-3"
				limited := "vless://limited@example:443#LIMITED-1" + separator + "trojan://limited@example:443#LIMITED-2"
				out, err := MergeBase64([]byte(encoding.EncodeToString([]byte(primary))), []byte(encoding.EncodeToString([]byte(limited))), "🇩🇪 A+1")
				if err != nil {
					t.Fatal(err)
				}
				want := anchorURI + separator + limited + "\n" + "trojan://secret@example:443#PRIMARY-2" + separator + "ss://secret@example:443#PRIMARY-3"
				if got := decodeMergedBase64(t, out); got != want {
					t.Fatalf("URI bytes or line endings changed: got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestMergeBase64SkipsInvalidNames(t *testing.T) {
	for _, primary := range []string{
		"vless://example:443",
		"vless://example:443#invalid%ZZ",
		"vless://example:443#OTHER",
	} {
		primary += "\nvless://example:443#OTHER"
		out, err := MergeBase64(encodeBase64(primary), encodeBase64("vless://limited:443#LIMITED"), "ANCHOR")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := decodeMergedBase64(t, out), primary+"\nvless://limited:443#LIMITED"; got != want {
			t.Fatalf("invalid or absent name matched: got %q, want %q", got, want)
		}
	}
}

func TestMergeXrayInsertionPreservesRawProfiles(t *testing.T) {
	anchor := json.RawMessage(`{"remarks":"ANCHOR","big":9007199254740993,"order":{"z":1,"a":2}}`)
	other := json.RawMessage(`{"remarks":123,"big":9007199254740995}`)
	limited := json.RawMessage(`{"remarks":"LIMITED GiB","order":{"z":2,"a":1}}`)
	out, err := MergeXray(marshalRawJSONArray([]json.RawMessage{anchor, other}), marshalRawJSONArray([]json.RawMessage{limited}), "ANCHOR")
	if err != nil {
		t.Fatal(err)
	}
	var profiles []json.RawMessage
	if err := json.Unmarshal(out, &profiles); err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 || !bytes.Equal(profiles[0], anchor) || !bytes.Equal(profiles[2], other) ||
		!bytes.Equal(profiles[1], []byte(`{"remarks":"LIMITED","order":{"z":2,"a":1}}`)) {
		t.Fatalf("raw objects were changed: %s", out)
	}
	out, err = MergeXray(marshalRawJSONArray([]json.RawMessage{other, anchor}), marshalRawJSONArray([]json.RawMessage{limited}), "123")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &profiles); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(profiles[1], anchor) {
		t.Fatal("non-string remarks matched an anchor")
	}
}

func TestMergeSingBoxInsertionUsesIndependentLists(t *testing.T) {
	primary := []byte(`{"route":{"final":"Select"},"outbounds":[
		{"type":"selector","tag":"Select","outbounds":["B","direct","A"]},
		{"type":"direct","tag":"direct"},
		{"type":"vless","tag":"A","server":"a.example"},
		{"type":"vless","tag":"B","server":"b.example"},
		{"type":"selector","tag":"Other","outbounds":["A","B"]}
	]}`)
	limited := singBoxConfig([]string{"LIMITED GiB"})
	for _, tc := range []struct {
		anchor string
		tags   []string
		refs   []string
	}{
		{"B", []string{"Select", "direct", "A", "B", "LIMITED", "Other"}, []string{"B", "LIMITED", "direct", "A"}},
		{"direct", []string{"Select", "direct", "A", "B", "Other", "LIMITED"}, []string{"B", "direct", "A", "LIMITED"}},
	} {
		out, err := MergeSingBox(primary, limited, tc.anchor)
		if err != nil {
			t.Fatal(err)
		}
		cfg := decodeObject(t, out)
		outbounds := cfg["outbounds"].([]any)
		var tags []string
		for _, item := range outbounds {
			tags = append(tags, item.(map[string]any)["tag"].(string))
			if item.(map[string]any)["tag"] == "Other" && !reflect.DeepEqual(item.(map[string]any)["outbounds"], []any{"A", "B"}) {
				t.Fatal("second selector changed")
			}
		}
		assertStringSliceEqual(t, "definitions", tags, tc.tags)
		var refs []string
		for _, ref := range outbounds[0].(map[string]any)["outbounds"].([]any) {
			refs = append(refs, ref.(string))
		}
		assertStringSliceEqual(t, "first selector", refs, tc.refs)
		if !reflect.DeepEqual(cfg["route"], map[string]any{"final": "Select"}) {
			t.Fatal("route changed")
		}
	}
	// A missing anchor appends in both lists and still validates references.
	if _, err := MergeSingBox(primary, limited, "MISSING"); err != nil {
		t.Fatal(err)
	}
	// Keep the existing error when a real primary proxy is absent from the selector.
	missingRef := bytes.Replace(primary, []byte(`"B","direct","A"`), []byte(`"direct","A"`), 1)
	if _, err := MergeSingBox(missingRef, limited, "B"); err == nil || !strings.Contains(err.Error(), "does not reference") {
		t.Fatalf("expected existing reference validation error, got %v", err)
	}
}

func TestMergeClashInsertionUsesIndependentListsAndUniqueReferences(t *testing.T) {
	primary := []byte(`proxies:
  - {name: A, type: vless, server: a.example}
  - {name: B, type: vless, server: b.example}
proxy-groups:
  - {name: Select, type: select, proxies: [B, DIRECT, A]}
  - {name: Other, type: select, proxies: [A, B]}
rules: [MATCH,Select]
`)
	limited := clashConfig([]string{"LIMITED GiB", "B", "LIMITED GiB"})
	for _, tc := range []struct {
		anchor string
		names  []string
		refs   []string
	}{
		{"B", []string{"A", "B", "LIMITED", "B", "LIMITED"}, []string{"B", "LIMITED", "DIRECT", "A"}},
		{"DIRECT", []string{"A", "B", "LIMITED", "B", "LIMITED"}, []string{"B", "DIRECT", "A", "LIMITED"}},
	} {
		out, err := MergeClashLike(primary, limited, tc.anchor)
		if err != nil {
			t.Fatal(err)
		}
		assertStringSliceEqual(t, "definitions", getProxiesNames(t, out), tc.names)
		assertStringSliceEqual(t, "first group", getFirstGroupProxies(t, out), tc.refs)
		assertStringSliceEqual(t, "second group", getGroupProxiesAt(t, out, 1), []string{"A", "B"})
		var cfg map[string]any
		if err := yaml.Unmarshal(out, &cfg); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg["rules"], []any{"MATCH", "Select"}) {
			t.Fatal("rules changed")
		}
	}
	missingRef := bytes.Replace(primary, []byte("[B, DIRECT, A]"), []byte("[B, DIRECT]"), 1)
	out, err := MergeClashLike(missingRef, clashConfig([]string{"LIMITED"}), "A")
	if err != nil {
		t.Fatal(err)
	}
	assertStringSliceEqual(t, "definitions", getProxiesNames(t, out), []string{"A", "LIMITED", "B"})
	assertStringSliceEqual(t, "missing group anchor", getFirstGroupProxies(t, out), []string{"B", "DIRECT", "LIMITED"})
}
