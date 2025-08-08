package joltsec

import (
    "bytes"
    "crypto/rand"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"

    "github.com/yourname/jolt-go/jolt"
)

const (
    magicJSEC = "JSEC"
    ver01     = 0x01
)

type Header struct {
    Alg   Alg               `json:"alg"`
    KeyID string            `json:"kid"`
    Extra map[string]string `json:"extra,omitempty"`
}

func EncryptJOLT(v any, hdr Header, kr Keyring) ([]byte, error) {
    suite, err := suiteFor(hdr.Alg); if err!=nil { return nil, err }
    key, err := kr.Get(hdr.KeyID); if err!=nil { return nil, err }
    if len(key) != suite.keyLen { return nil, fmt.Errorf("key length %d mismatch for %s", len(key), hdr.Alg) }

    pt, err := jolt.EncodeBinary(v); if err!=nil { return nil, err }

    nonce := make([]byte, suite.nonceLen)
    if _, err := io.ReadFull(rand.Reader, nonce); err!=nil { return nil, err }

    a, err := suite.newAEAD(key); if err!=nil { return nil, err }

    if hdr.Extra == nil { hdr.Extra = map[string]string{} }
    aadJSON, err := json.Marshal(hdr); if err!=nil { return nil, err }

    ct := a.Seal(nil, nonce, pt, aadJSON)

    var buf bytes.Buffer
    buf.WriteString(magicJSEC)
    buf.WriteByte(ver01)
    writeVarBytes(&buf, []byte(hdr.Alg))
    writeVarBytes(&buf, []byte(hdr.KeyID))
    writeVarBytes(&buf, nonce)
    writeVarBytes(&buf, aadJSON)
    writeVarBytes(&buf, ct)
    return buf.Bytes(), nil
}

func DecryptJOLT(jsec []byte, kr Keyring) (any, Header, error) {
    rd := bytes.NewReader(jsec)
    magic := make([]byte, 4)
    if _, err := io.ReadFull(rd, magic); err!=nil { return nil, Header{}, err }
    if string(magic) != magicJSEC { return nil, Header{}, fmt.Errorf("bad magic") }
    ver, err := rd.ReadByte(); if err!=nil { return nil, Header{}, err }
    if ver != ver01 { return nil, Header{}, fmt.Errorf("unsupported version %d", ver) }

    alg, err := readVarBytes(rd); if err!=nil { return nil, Header{}, err }
    keyID, err := readVarBytes(rd); if err!=nil { return nil, Header{}, err }
    nonce, err := readVarBytes(rd); if err!=nil { return nil, Header{}, err }
    aadJSON, err := readVarBytes(rd); if err!=nil { return nil, Header{}, err }
    ct, err := readVarBytes(rd); if err!=nil { return nil, Header{}, err }

    var hdr Header
    if err := json.Unmarshal(aadJSON, &hdr); err!=nil { return nil, Header{}, err }
    if hdr.KeyID != string(keyID) || string(alg) != string(hdr.Alg) {
        return nil, Header{}, fmt.Errorf("AAD/header mismatch")
    }

    suite, err := suiteFor(hdr.Alg); if err!=nil { return nil, Header{}, err }
    key, err := kr.Get(hdr.KeyID); if err!=nil { return nil, Header{}, err }
    if len(key) != suite.keyLen { return nil, Header{}, fmt.Errorf("key length mismatch") }

    a, err := suite.newAEAD(key); if err!=nil { return nil, Header{}, err }
    pt, err := a.Open(nil, nonce, ct, aadJSON); if err!=nil { return nil, Header{}, fmt.Errorf("decryption failed: %w", err) }

    v, err := jolt.DecodeBinary(pt)
    return v, hdr, err
}

func writeVarBytes(w io.Writer, b []byte) {
    var hdr [10]byte
    n := binary.PutUvarint(hdr[:], uint64(len(b)))
    w.Write(hdr[:n]); w.Write(b)
}
func readVarBytes(r io.ByteReader) ([]byte, error) {
    n, err := binary.ReadUvarint(r); if err!=nil { return nil, err }
    b := make([]byte, n)
    for i:=range b { bt,e := r.ReadByte(); if e!=nil { return nil, e }; b[i]=bt }
    return b, nil
}
