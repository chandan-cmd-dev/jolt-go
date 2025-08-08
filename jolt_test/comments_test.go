package jolt_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/yourname/jolt-go/jolt"
)

func TestJSONCommentsAccepted(t *testing.T) {
	src := []byte(`{
    // order doc
    "$meta": { "type": "t", "version": "1" }, /* meta ok */
    "$body": {
      "$comment": "human note",
      "n": { "@type":"int", "value":"2" } // trailing comment
    }
  }`)
	var v any
	if err := jolt.UnmarshalJSONWithComments(src, &v); err != nil {
		t.Fatalf("unmarshal with comments failed: %v", err)
	}
	bin, err := jolt.EncodeBinary(v)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jolt.DecodeBinary(bin)
	if err != nil {
		t.Fatal(err)
	}
	js, _ := jolt.MarshalJSONCompat(out, true)
	if bytes.Contains(js, []byte(`"$comment"`)) {
		t.Fatal("expected $comment to be dropped by default")
	}
}

func TestPreserveCommentsFlag(t *testing.T) {
	src := []byte(`{"$body":{"$comment":"keep me","a":1}}`)
	var v any
	if err := jolt.UnmarshalJSONWithComments(src, &v); err != nil {
		t.Fatal(err)
	}

	jolt.PreserveComments = true
	defer func() { jolt.PreserveComments = false }()

	// Round-trip through the binary codec
	bin, err := jolt.EncodeBinary(v)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jolt.DecodeBinary(bin)
	if err != nil {
		t.Fatal(err)
	}

	// Debug: show what actually came back
	js, _ := jolt.MarshalJSONCompat(out, false)
	t.Logf("roundtrip JSON: %s", js)

	if !bytes.Contains(js, []byte(`"$comment"`)) {
		t.Fatal("expected $comment to be preserved when flag is true")
	}
}

func TestStripJSONCommentsStringSafety(t *testing.T) {
	src := []byte(`{"x": "not // a comment", "y": "/* not a block */"}`)
	var v any
	if err := json.Unmarshal(jolt.StripJSONComments(src), &v); err != nil {
		t.Fatalf("stripper broke JSON: %v", err)
	}
}
