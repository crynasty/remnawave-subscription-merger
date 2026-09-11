package merge

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// loadFile reads a fixture and fails the test if it cannot be loaded
func loadFile(t *testing.T, path string) []byte {
	t.Helper()
	fixturePath := filepath.Join("testdata", filepath.Base(path))
	b, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("loadFile %s: %v", fixturePath, err)
	}
	return b
}

// Helpers for inspecting yaml.Node values.
func getProxiesNames(t *testing.T, out []byte) []string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	root := doc.Content[0]
	proxiesNode := mappingGetValue(root, "proxies")
	if proxiesNode == nil {
		t.Fatal("result has no 'proxies'")
	}
	names, err := extractProxyNames(proxiesNode)
	if err != nil {
		t.Fatalf("extractProxyNames: %v", err)
	}
	return names
}

func getFirstGroupProxies(t *testing.T, out []byte) []string {
	t.Helper()
	return getGroupProxiesAt(t, out, 0)
}

func getGroupProxiesAt(t *testing.T, out []byte, index int) []string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	root := doc.Content[0]
	groupsNode := mappingGetValue(root, "proxy-groups")
	if groupsNode == nil {
		t.Fatal("result has no 'proxy-groups'")
	}
	if index >= len(groupsNode.Content) {
		t.Fatalf("result has %d proxy-groups, want index %d", len(groupsNode.Content), index)
	}
	group := groupsNode.Content[index]
	proxiesNode := mappingGetValue(group, "proxies")
	if proxiesNode == nil {
		t.Fatalf("proxy-group %d has no 'proxies'", index)
	}
	var names []string
	for _, n := range proxiesNode.Content {
		names = append(names, n.Value)
	}
	return names
}

func assertStringSliceEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got %d %v, want %d %v", label, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d]: got %q in %v, want %q in %v", label, i, got[i], got, want[i], want)
		}
	}
}

// TestMergeClash: primary config = clash.yml, limited config = mihomo.yml.
// Both formats are identical, so Mihomo serves as the realistic limited config.
func TestMergeClash(t *testing.T) {
	primaryConfig := loadFile(t, "./clash.yml")
	limitedConfig := loadFile(t, "./mihomo.yml")

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	proxyNames := getProxiesNames(t, out)
	t.Logf("proxies after merge (%d): %v", len(proxyNames), proxyNames)
	assertStringSliceEqual(t, "proxies", proxyNames, []string{"Kazakhstan VLESS", "Germany VLESS"})

	groupProxies := getFirstGroupProxies(t, out)
	t.Logf("first proxy-group proxies: %v", groupProxies)
	assertStringSliceEqual(t, "first group proxies", groupProxies, []string{"Kazakhstan VLESS", "Germany VLESS"})
}

// TestMergeMihomo: primary config = mihomo.yml, limited config = clash.yml.
func TestMergeMihomo(t *testing.T) {
	primaryConfig := loadFile(t, "./mihomo.yml")
	limitedConfig := loadFile(t, "./clash.yml")

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	proxyNames := getProxiesNames(t, out)
	groupProxies := getFirstGroupProxies(t, out)

	t.Logf("proxies: %v", proxyNames)
	t.Logf("first group proxies: %v", groupProxies)

	assertStringSliceEqual(t, "proxies", proxyNames, []string{"Germany VLESS", "Kazakhstan VLESS"})
	assertStringSliceEqual(t, "first group proxies", groupProxies, []string{"Germany VLESS", "Kazakhstan VLESS"})
}

// TestMergeStash: primary config = stash.yml, limited config = mihomo.yml.
func TestMergeStash(t *testing.T) {
	primaryConfig := loadFile(t, "./stash.yml")
	limitedConfig := loadFile(t, "./mihomo.yml")

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	proxyNames := getProxiesNames(t, out)
	groupProxies := getFirstGroupProxies(t, out)

	t.Logf("proxies: %v", proxyNames)
	t.Logf("first group proxies: %v", groupProxies)

	assertStringSliceEqual(t, "proxies", proxyNames, []string{"Netherlands VLESS", "Germany VLESS"})
	assertStringSliceEqual(t, "first group proxies", groupProxies, []string{"Netherlands VLESS", "Germany VLESS"})
}

// TestMergeClashLike_PreservesPrimaryKeys verifies that the template's other keys
// (rules, DNS, TUN, and so on) are preserved after the merge.
func TestMergeClashLike_PreservesPrimaryKeys(t *testing.T) {
	primaryConfig := loadFile(t, "./mihomo.yml")
	limitedConfig := loadFile(t, "./clash.yml")

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	outStr := string(out)
	mustContain := []string{"rules:", "dns:", "tun:", "sniffer:", "rule-providers:"}
	for _, key := range mustContain {
		if !strings.Contains(outStr, key) {
			t.Errorf("result missing expected key: %q", key)
		}
	}
}

// TestMergeClashLike_MultipleProxies verifies merging a limited config with multiple proxies.
func TestMergeClashLike_MultipleProxies(t *testing.T) {
	// Build a limited config with multiple proxies.
	multiLimited := []byte(`
proxies:
  - name: 🇰🇿 Kazakhstan VLESS
    type: vless
    server: kz.example.com
    port: 443
    uuid: aaa
  - name: 🇩🇪 Germany VLESS
    type: vless
    server: de.example.com
    port: 443
    uuid: bbb
  - name: 🇳🇱 Netherlands VMess
    type: vmess
    server: nl.example.com
    port: 443
    uuid: ccc
proxy-groups:
  - name: Limited Group
    type: select
    proxies: []
rules: []
`)
	primaryConfig := loadFile(t, "./mihomo.yml")

	out, err := MergeClashLike(primaryConfig, multiLimited)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	proxyNames := getProxiesNames(t, out)
	groupProxies := getFirstGroupProxies(t, out)

	expected := []string{"Germany VLESS", "🇰🇿 Kazakhstan VLESS", "🇩🇪 Germany VLESS", "🇳🇱 Netherlands VMess"}
	assertStringSliceEqual(t, "proxies", proxyNames, expected)
	assertStringSliceEqual(t, "first group proxies", groupProxies, expected)

	t.Logf("proxies: %v", proxyNames)
	t.Logf("first group: %v", groupProxies)
}

// TestMergeClashLike_FirstProxyGroupByOrder verifies that the main group is selected
// from the first proxy-groups[].proxies list rather than by group name.
func TestMergeClashLike_FirstProxyGroupByOrder(t *testing.T) {
	primaryConfig := []byte(`
proxies:
  - name: primary A
    type: vless
    server: primary.example.com
    port: 443
    uuid: aaa
proxy-groups:
  - name: Any Primary Name
    type: select
    proxies:
      - primary A
  - name: CRYNASTY VPN
    type: select
    proxies:
      - primary A
rules:
  - MATCH,Any Primary Name
`)
	limitedConfig := []byte(`
proxies:
  - name: Limited B
    type: vless
    server: limited.example.com
    port: 443
    uuid: bbb
proxy-groups:
  - name: Limited Group
    type: select
    proxies:
      - Limited B
rules: []
`)

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	assertStringSliceEqual(t, "proxies", getProxiesNames(t, out), []string{"primary A", "Limited B"})
	assertStringSliceEqual(t, "first group proxies", getGroupProxiesAt(t, out, 0), []string{"primary A", "Limited B"})
	assertStringSliceEqual(t, "second group proxies", getGroupProxiesAt(t, out, 1), []string{"primary A"})
}

// TestMergeClashLike_OutputValid verifies that the output is valid YAML.
func TestMergeClashLike_OutputValid(t *testing.T) {
	primaryConfig := loadFile(t, "./clash.yml")
	limitedConfig := loadFile(t, "./mihomo.yml")

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}

	// Print the merged config for visual inspection.
	fmt.Printf("\n=== Merged Clash config ===\n%s\n=== end ===\n", string(out))
}

func TestMergeClashLike_TwoSpaceIndentAndSemanticGolden(t *testing.T) {
	primaryConfig := []byte(`
dns:
  enable: true
  nameserver:
    - 1.1.1.1
proxies:
  - name: primary
    type: trojan
    server: primary.example
    port: 443
    password: secret-a
proxy-groups:
  - name: Main
    type: select
    proxies:
      - primary
rules:
  - MATCH,Main
`)
	limitedConfig := []byte(`
proxies:
  - name: Limited
    type: vless
    server: limited.example
    port: 443
    uuid: 00000000-0000-0000-0000-000000000001
`)
	wantYAML := []byte(`
dns:
  enable: true
  nameserver:
    - 1.1.1.1
proxies:
  - name: primary
    type: trojan
    server: primary.example
    port: 443
    password: secret-a
  - name: Limited
    type: vless
    server: limited.example
    port: 443
    uuid: 00000000-0000-0000-0000-000000000001
proxy-groups:
  - name: Main
    type: select
    proxies:
      - primary
      - Limited
rules:
  - MATCH,Main
`)

	out, err := MergeClashLike(primaryConfig, limitedConfig)
	if err != nil {
		t.Fatalf("MergeClashLike: %v", err)
	}
	if strings.Contains(string(out), "\n    enable:") {
		t.Fatalf("output still uses doubled mapping indentation:\n%s", out)
	}

	var got, want any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(wantYAML, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("two-space serialization changed configuration data\nwant: %#v\n got: %#v", want, got)
	}
}
