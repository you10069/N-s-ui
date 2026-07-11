package util

import (
	"encoding/json"
	"testing"
)

func TestStripJSONComments(t *testing.T) {
	input := []byte(`{
  // comment
  "url": "https://example.com/a//b",
  /* block */
  "value": "/* text */"
}`)
	clean, err := StripJSONComments(input)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if err = json.Unmarshal(clean, &value); err != nil {
		t.Fatal(err)
	}
	if value["url"] != "https://example.com/a//b" || value["value"] != "/* text */" {
		t.Fatalf("unexpected decoded value: %#v", value)
	}
}
