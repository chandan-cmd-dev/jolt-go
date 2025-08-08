package jolt_test

import (
  "bufio"
  "bytes"
  "encoding/base64"
  "encoding/json"
  "os"
  "path/filepath"
  "strings"
  "testing"

  "github.com/chandan-cmd-dev/jolt-go/jolt"
)

func TestRoundTrip(t *testing.T) {
  u, _ := jolt.NewUUID()
  env := jolt.Envelope{
    Meta: jolt.Meta{Type:"urn:jolt:example/Order", Version:"2.1.0", Created: jolt.Ptr(jolt.TSNowUTC())},
    Body: map[string]any{
      "$id":"order:9f2e",
      "number":"SO-12988",
      "qty": jolt.BigInt(2),
      "price": mustDec("1999.95"),
      "uuid": u,
      "tags": jolt.Set{"gift","festival"},
    },
  }
  bin, err := jolt.EncodeBinary(env); if err!=nil { t.Fatal(err) }
  out, err := jolt.DecodeBinary(bin); if err!=nil { t.Fatal(err) }
  js1, _ := jolt.MarshalJSONCompat(env, true)
  js2, _ := jolt.MarshalJSONCompat(out, true)
  if !bytes.Equal(js1, js2) { t.Fatalf("roundtrip mismatch\n%s\n%s", js1, js2) }

  var buf bytes.Buffer
  if err := jolt.WriteFrame(&buf, bin); err!=nil { t.Fatal(err) }
  rd := bufio.NewReader(&buf)
  frame, err := jolt.ReadFrame(rd); if err!=nil { t.Fatal(err) }
  if !bytes.Equal(frame, bin) { t.Fatal("frame mismatch") }
}

func TestGoldenEncoding(t *testing.T) {
  var v any
  if err := json.Unmarshal(orderJSON(), &v); err != nil {
    t.Fatalf("bad fixture: %v", err)
  }
  got, err := jolt.EncodeBinary(v); if err != nil { t.Fatal(err) }

  path := filepath.Join("testdata", "order.jb.golden.b64")
  if _, err := os.Stat(path); os.IsNotExist(err) {
    _ = os.MkdirAll(filepath.Dir(path), 0o755)
    if werr := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(got)+"\n"), 0o644); werr != nil {
      t.Fatalf("write golden: %v", werr)
    }
  }
  wantB64, err := os.ReadFile(path); if err != nil { t.Fatal(err) }
  want, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(wantB64))); if err != nil { t.Fatal(err) }
  if !bytes.Equal(got, want) {
    t.Fatalf("golden changed got=%d want=%d", len(got), len(want))
  }
}

func BenchmarkEncode(b *testing.B) {
  var v any
  _ = json.Unmarshal(orderJSON(), &v)
  b.ReportAllocs()
  for i:=0;i<b.N;i++ { _, _ = jolt.EncodeBinary(v) }
}
func BenchmarkDecode(b *testing.B) {
  var v any
  _ = json.Unmarshal(orderJSON(), &v)
  got, _ := jolt.EncodeBinary(v)
  path := filepath.Join("testdata", "order.jb.golden.b64")
  if _, err := os.Stat(path); os.IsNotExist(err) {
    _ = os.MkdirAll(filepath.Dir(path), 0o755)
    _ = os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(got)+"\n"), 0o644)
  }
  raw, _ := os.ReadFile(path)
  blob, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
  b.ReportAllocs()
  for i:=0;i<b.N;i++ { _, _ = jolt.DecodeBinary(blob) }
}
