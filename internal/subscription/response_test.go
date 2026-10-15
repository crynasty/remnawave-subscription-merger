package subscription

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/crynasty/remnawave-subscription-merger/internal/detector"
	"gopkg.in/yaml.v3"
)

func TestMergeConfigsPassesInsertionAnchorToEveryFormat(t *testing.T) {
	primaryYAML := []byte(`proxies:
  - {name: PRIMARY-1, type: vless, server: p1.example}
  - {name: PRIMARY-2, type: vless, server: p2.example}
proxy-groups:
  - {name: Select, type: select, proxies: [PRIMARY-1, PRIMARY-2]}
`)
	limitedYAML := []byte(`proxies:
  - {name: LIMITED-1, type: vless, server: l.example}
`)
	encode := func(value string) []byte { return []byte(base64.StdEncoding.EncodeToString([]byte(value))) }
	for _, tc := range []struct {
		responseType detector.ResponseType
		primary      []byte
		limited      []byte
	}{
		{detector.ResponseXrayJSON, []byte(`[{"remarks":"PRIMARY-1"},{"remarks":"PRIMARY-2"}]`), []byte(`[{"remarks":"LIMITED-1"}]`)},
		{detector.ResponseBase64, encode("vless://p1.example:443#PRIMARY-1\nvless://p2.example:443#PRIMARY-2"), encode("vless://l.example:443#LIMITED-1")},
		{detector.ResponseSingBox, []byte(`{"outbounds":[
			{"type":"selector","tag":"Select","outbounds":["PRIMARY-1","PRIMARY-2"]},
			{"type":"vless","tag":"PRIMARY-1","server":"p1.example"},
			{"type":"vless","tag":"PRIMARY-2","server":"p2.example"}
		]}`), []byte(`{"outbounds":[{"type":"vless","tag":"LIMITED-1","server":"l.example"}]}`)},
		{detector.ResponseClash, primaryYAML, limitedYAML},
		{detector.ResponseMihomo, primaryYAML, limitedYAML},
		{detector.ResponseStash, primaryYAML, limitedYAML},
	} {
		t.Run(tc.responseType.String(), func(t *testing.T) {
			for _, anchor := range []string{"PRIMARY-1", "", "MISSING"} {
				t.Run(anchor, func(t *testing.T) {
					out, err := mergeConfigs(tc.primary, tc.limited, tc.responseType, anchor)
					if err != nil {
						t.Fatal(err)
					}
					var names, references []string
					switch tc.responseType {
					case detector.ResponseXrayJSON:
						var profiles []struct{ Remarks string }
						if err := json.Unmarshal(out, &profiles); err != nil {
							t.Fatal(err)
						}
						for _, profile := range profiles {
							names = append(names, profile.Remarks)
						}
					case detector.ResponseBase64:
						plain, err := base64.StdEncoding.DecodeString(string(out))
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range strings.Split(string(plain), "\n") {
							_, name, _ := strings.Cut(line, "#")
							names = append(names, name)
						}
					case detector.ResponseSingBox:
						var cfg struct {
							Outbounds []struct {
								Tag       string
								Server    string
								Outbounds []string
							}
						}
						if err := json.Unmarshal(out, &cfg); err != nil {
							t.Fatal(err)
						}
						for _, outbound := range cfg.Outbounds {
							if outbound.Server != "" {
								names = append(names, outbound.Tag)
							} else {
								references = outbound.Outbounds
							}
						}
					default:
						var cfg struct {
							Proxies []struct{ Name string }
							Groups  []struct{ Proxies []string } `yaml:"proxy-groups"`
						}
						if err := yaml.Unmarshal(out, &cfg); err != nil {
							t.Fatal(err)
						}
						for _, proxy := range cfg.Proxies {
							names = append(names, proxy.Name)
						}
						references = cfg.Groups[0].Proxies
					}
					want := []string{"PRIMARY-1", "PRIMARY-2", "LIMITED-1"}
					if anchor == "PRIMARY-1" {
						want = []string{"PRIMARY-1", "LIMITED-1", "PRIMARY-2"}
					}
					if !reflect.DeepEqual(names, want) || (references != nil && !reflect.DeepEqual(references, want)) {
						t.Fatalf("anchor %q: definitions %v, references %v; want %v", anchor, names, references, want)
					}
				})
			}
		})
	}
}
