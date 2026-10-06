package merge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeXrayCleansOnlyLimitedRemarks(t *testing.T) {
	primary := []byte(`[{"remarks":"Primary 10 GiB","customNumber":9007199254740993}]`)
	limited := []byte(`[
  {"remarks" : "🇩🇪 1\u0020GiB / 20 GiB", "customNumber":9007199254740993,"nested":{"remarks":"keep GiB"},"tag":"keep GiB"},
  {"remarks":"GiB 2GiB 3 gib 4 GB","customOrder":{"z":1,"a":2}}
]`)
	out, err := MergeXray(primary, limited)
	if err != nil {
		t.Fatal(err)
	}
	var profiles []json.RawMessage
	if err := json.Unmarshal(out, &profiles); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(profiles[0], []byte(`{"remarks":"Primary 10 GiB","customNumber":9007199254740993}`)) {
		t.Fatalf("primary profile changed: %s", profiles[0])
	}
	want := `{"remarks" : "🇩🇪 1 / 20", "customNumber":9007199254740993,"nested":{"remarks":"keep GiB"},"tag":"keep GiB"}`
	if !bytes.Equal(profiles[1], []byte(want)) {
		t.Fatalf("limited profile mismatch\nwant: %s\n got: %s", want, profiles[1])
	}
	if !bytes.Equal(profiles[2], []byte(`{"remarks":"GiB 2GiB 3 gib 4 GB","customOrder":{"z":1,"a":2}}`)) {
		t.Fatalf("unmatched name changed: %s", profiles[2])
	}
}

func TestMergeSingBoxCleansLimitedTagsAndSelectorReferences(t *testing.T) {
	primary := []byte(`{"outbounds":[
  {"type":"selector","tag":"Select GiB","outbounds":["Primary 10 GiB"]},
  {"type":"vless","tag":"Primary 10 GiB","server":"primary.example"}
]}`)
	limited := []byte(`{"outbounds":[
  {"type":"vless","tag":"Limited 1 GiB / 20 GiB","server":"keep GiB","password":"keep GiB"},
  {"type":"trojan","tag":"GiB 2GiB 3 gib","server":"b.example"},
  {"type":"direct","tag":"direct GiB"}
]}`)
	out, err := MergeSingBox(primary, limited)
	if err != nil {
		t.Fatal(err)
	}
	tags, selector := singBoxTags(t, out)
	assertStringSliceEqual(t, "tags", tags, []string{"Select GiB", "Primary 10 GiB", "Limited 1 / 20", "GiB 2GiB 3 gib"})
	assertStringSliceEqual(t, "selector", selector, []string{"Primary 10 GiB", "Limited 1 / 20", "GiB 2GiB 3 gib"})
	proxy := decodeObject(t, out)["outbounds"].([]any)[2].(map[string]any)
	if proxy["server"] != "keep GiB" || proxy["password"] != "keep GiB" {
		t.Fatalf("non-name fields changed: %v", proxy)
	}
}

func TestMergeClashLikeCleansLimitedNamesAndGroupReferences(t *testing.T) {
	primary := []byte(`proxies:
  - name: Primary 10 GiB
    type: trojan
    server: primary.example
proxy-groups:
  - name: Select GiB
    type: select
    proxies: [Primary 10 GiB]
rules: [MATCH,Select GiB]
`)
	limited := []byte(`proxies:
  - name: Limited 1 GiB / 20 GiB
    type: trojan
    server: keep GiB
    password: keep GiB
  - name: GiB 2GiB 3 gib
    type: vless
    server: b.example
`)
	out, err := MergeClashLike(primary, limited)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Primary 10 GiB", "Limited 1 / 20", "GiB 2GiB 3 gib"}
	assertStringSliceEqual(t, "names", getProxiesNames(t, out), want)
	assertStringSliceEqual(t, "group", getGroupProxiesAt(t, out, 0), want)
	for _, field := range []string{"name: Select GiB", "server: keep GiB", "password: keep GiB"} {
		if !strings.Contains(string(out), field) {
			t.Fatalf("lost unchanged field %q in %s", field, out)
		}
	}
}

func TestMergeBase64CleansOnlyLimitedURINames(t *testing.T) {
	vmessRaw := `{"v":"2","ps":"🇩🇪 1 GiB / 20 GiB","add":"keep GiB","customNumber":9007199254740993}`
	vmessClean := `{"v":"2","ps":"🇩🇪 1 / 20","add":"keep GiB","customNumber":9007199254740993}`
	vmess := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessRaw))
	vmessUnpadded := "vmess://" + base64.RawStdEncoding.EncodeToString([]byte(vmessRaw))
	primary := "vless://primary.example:443#Primary%2010%20GiB\n" + vmess
	limited := strings.Join([]string{
		"vless://id@limited.example:443?other=keep%20GiB#Limited%201%20GiB%20/%2020%20GiB",
		"trojan://keep%20GiB@limited.example:443#Limited 2 GiB",
		"ss://secret@limited.example:443#A+1%20GiB",
		"hysteria2://limited.example:443#Encoded%20%47%69%42",
		"tuic://limited.example:443#GiB%202GiB%203%20gib",
		"vless://limited.example:443?other=keep%20GiB",
		"vless://limited.example:443#invalid%ZZ%20GiB",
		vmess,
		vmessUnpadded,
	}, "\r\n")
	out, err := MergeBase64(encodeBase64(primary), encodeBase64(limited))
	if err != nil {
		t.Fatal(err)
	}
	want := primary + "\n" + strings.Join([]string{
		"vless://id@limited.example:443?other=keep%20GiB#Limited%201%20/%2020",
		"trojan://keep%20GiB@limited.example:443#Limited%202",
		"ss://secret@limited.example:443#A+1",
		"hysteria2://limited.example:443#Encoded",
		"tuic://limited.example:443#GiB%202GiB%203%20gib",
		"vless://limited.example:443?other=keep%20GiB",
		"vless://limited.example:443#invalid%ZZ%20GiB",
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessClean)),
		"vmess://" + base64.RawStdEncoding.EncodeToString([]byte(vmessClean)),
	}, "\r\n")
	if got := decodeMergedBase64(t, out); got != want {
		t.Fatalf("URI names mismatch\nwant: %s\n got: %s", want, got)
	}
}
