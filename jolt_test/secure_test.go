package jolt_test

import (
  "bytes"
  "encoding/base64"
  "encoding/binary"
  "encoding/json"
  "os"
  "path/filepath"
  "strings"
  "testing"

  "golang.org/x/crypto/chacha20poly1305"

  "github.com/chandan-cmd-dev/jolt-go/jolt"
  "github.com/chandan-cmd-dev/jolt-go/joltsec"
)

func TestEncryptDecrypt_XChaCha(t *testing.T) {
  env := jolt.Envelope{
    Meta: jolt.Meta{Type:"urn:jolt:example/Order", Version:"2.1.0"},
    Body: map[string]any{"n": jolt.BigInt(42), "p": mustDec("1234.56"), "ts": jolt.TSNowUTC()},
  }
  key := make([]byte, 32); for i:=range key { key[i]=byte(i) }
  kr := joltsec.StaticKeyring{"k1": key}
  hdr := joltsec.Header{Alg: joltsec.AlgXChaCha20Poly1305, KeyID:"k1", Extra: map[string]string{"ctx":"demo"}}

  blob, err := joltsec.EncryptJOLT(env, hdr, kr); if err!=nil { t.Fatal(err) }
  out, outHdr, err := joltsec.DecryptJOLT(blob, kr); if err!=nil { t.Fatal(err) }
  if outHdr.KeyID != "k1" { t.Fatal("bad keyid") }

  js1, _ := jolt.MarshalJSONCompat(env, true)
  js2, _ := jolt.MarshalJSONCompat(out, true)
  if !bytes.Equal(js1, js2) { t.Fatalf("mismatch") }
}

// Deterministic golden using fixed nonce (24 zero bytes). Not for production.
func TestGoldenEncrypted(t *testing.T) {
  var v any
  if err := json.Unmarshal(orderJSON(), &v); err != nil { t.Fatal(err) }

  pt, err := jolt.EncodeBinary(v); if err != nil { t.Fatal(err) }

  key := make([]byte, 32); for i:=range key { key[i]=byte(i) }
  nonce := make([]byte, chacha20poly1305.NonceSizeX)
  aead, err := chacha20poly1305.NewX(key); if err != nil { t.Fatal(err) }

  hdr := joltsec.Header{Alg: joltsec.AlgXChaCha20Poly1305, KeyID:"k1", Extra: map[string]string{"ctx":"golden"}}
  aad, _ := json.Marshal(hdr)
  ct := aead.Seal(nil, nonce, pt, aad)

  var buf bytes.Buffer
  buf.WriteString("JSEC"); buf.WriteByte(0x01)
  writeVarBytes(&buf, []byte(hdr.Alg))
  writeVarBytes(&buf, []byte(hdr.KeyID))
  writeVarBytes(&buf, nonce)
  writeVarBytes(&buf, aad)
  writeVarBytes(&buf, ct)
  sec := buf.Bytes()

  path := filepath.Join("testdata", "order.sec.golden.b64")
  if _, err := os.Stat(path); os.IsNotExist(err) {
    _ = os.MkdirAll(filepath.Dir(path), 0o755)
    if werr := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(sec)+"\n"), 0o644); werr != nil {
      t.Fatalf("write golden: %v", werr)
    }
  }
  wantB64, err := os.ReadFile(path); if err != nil { t.Fatal(err) }
  want, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(wantB64))); if err != nil { t.Fatal(err) }

  if !bytes.Equal(sec, want) { t.Fatalf("encrypted golden mismatch") }

  kr := joltsec.StaticKeyring{"k1": key}
  out, hdr2, err := joltsec.DecryptJOLT(sec, kr); if err != nil { t.Fatal(err) }
  if hdr2.KeyID != "k1" { t.Fatal("header mismatch") }
  _, _ = jolt.MarshalJSONCompat(out, true)
}

func writeVarBytes(w *bytes.Buffer, b []byte) {
  var hdr [10]byte
  n := binary.PutUvarint(hdr[:], uint64(len(b)))
  w.Write(hdr[:n]); w.Write(b)
}
