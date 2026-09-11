package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestMergeXrayPreservesRawProfilesAndAddsVariableCount(t *testing.T) {
	primaryConfig := []byte(`[
  {"remarks":"primary A","customNumber":9007199254740993,"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"a.example"}]},"tag":"proxy"}]},
  {"remarks":"primary B","customOrder":{"z":1,"a":2},"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"b.example"}]},"tag":"proxy"}]}
]`)

	for _, limitedCount := range []int{1, 3, 10} {
		t.Run(fmt.Sprintf("limited_%d", limitedCount), func(t *testing.T) {
			var limitedConfig bytes.Buffer
			limitedConfig.WriteByte('[')
			for i := 0; i < limitedCount; i++ {
				if i > 0 {
					limitedConfig.WriteByte(',')
				}
				fmt.Fprintf(&limitedConfig, `{"remarks":"Limited %d","marker":"raw-%d","outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"limited-%d.example"}]},"tag":"proxy"}]}`, i, i, i)
			}
			limitedConfig.WriteByte(']')

			out, err := MergeXray(primaryConfig, limitedConfig.Bytes())
			if err != nil {
				t.Fatalf("MergeXray: %v", err)
			}
			var profiles []json.RawMessage
			if err := json.Unmarshal(out, &profiles); err != nil {
				t.Fatalf("result is not a JSON array: %v", err)
			}
			if got, want := len(profiles), 2+limitedCount; got != want {
				t.Fatalf("profile count: got %d, want %d", got, want)
			}
			if !bytes.Contains(out, []byte(`"customNumber":9007199254740993`)) ||
				!bytes.Contains(out, []byte(`"customOrder":{"z":1,"a":2}`)) {
				t.Fatal("raw primary field representation was changed")
			}
		})
	}
}
