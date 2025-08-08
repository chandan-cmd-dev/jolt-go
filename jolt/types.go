package jolt

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/binary"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "math/big"
    "time"

    "github.com/cockroachdb/apd/v3"
    "github.com/yourname/jolt-go/jolt/internal/apdctx"
)

// ----- Scalars -----

type Int struct{ V *big.Int }
func BigInt(v int64) Int { return Int{V: big.NewInt(v)} }
func IntFromString(s string) (Int, error) {
    z := new(big.Int)
    if _, ok := z.SetString(s, 10); !ok { return Int{}, errors.New("bad bigint") }
    return Int{V: z}, nil
}

type Decimal struct{ D apd.Decimal }
func DecFromString(s string) (Decimal, error) {
    var d apd.Decimal
    _, _, err := apdctx.Ctx.SetString(&d, s)
    return Decimal{D: d}, err
}
func DecFromInt64(n int64) Decimal {
    var d apd.Decimal
    _, _, _ = apdctx.Ctx.SetString(&d, fmt.Sprintf("%d", n))
    return Decimal{D: d}
}
func (d Decimal) String() string { return d.D.String() }

type Binary []byte
type UUID [16]byte

func NewUUID() (UUID, error) {
    var u UUID
    if _, err := io.ReadFull(rand.Reader, u[:]); err != nil { return u, err }
    u[6] = (u[6] & 0x0f) | 0x40
    u[8] = (u[8] & 0x3f) | 0x80
    return u, nil
}
func (u UUID) String() string {
    b := u
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
        binary.BigEndian.Uint32(b[0:4]),
        binary.BigEndian.Uint16(b[4:6]),
        binary.BigEndian.Uint16(b[6:8]),
        binary.BigEndian.Uint16(b[8:10]),
        b[10:16])
}

type Link struct{ Ref string }
type Annot struct{ Note string }

type Timestamp struct{ RFC3339 string }
type Date struct{ YYYYMMDD string }
type Time struct{ HHMMSS string }

func TS(t time.Time) Timestamp { return Timestamp{RFC3339: t.Format(time.RFC3339Nano)} }
func TSNowUTC() Timestamp      { return TS(time.Now().UTC()) }
func DateYMD(y int, m time.Month, d int) Date { return Date{YYYYMMDD: fmt.Sprintf("%04d-%02d-%02d", y, m, d)} }
func TimeHMS(h, m, s int) Time { return Time{HHMMSS: fmt.Sprintf("%02d:%02d:%02d", h, m, s)} }
func Ptr[T any](v T) *T { return &v }

type Set []any
type Map map[any]any

// ----- JSON compatibility -----

func (i Int) MarshalJSON() ([]byte, error)       { return json.Marshal(map[string]any{"@type":"int","value": i.V.String()}) }
func (d Decimal) MarshalJSON() ([]byte, error)   { return json.Marshal(map[string]any{"@type":"dec","value": d.String()}) }
func (b Binary) MarshalJSON() ([]byte, error)    { return json.Marshal(map[string]any{"@type":"bin","value": base64.StdEncoding.EncodeToString(b)}) }
func (u UUID) MarshalJSON() ([]byte, error)      { return json.Marshal(map[string]any{"@type":"uuid","value": u.String()}) }
func (l Link) MarshalJSON() ([]byte, error)      { return json.Marshal(map[string]any{"@type":"link","ref": l.Ref}) }
func (a Annot) MarshalJSON() ([]byte, error)     { return json.Marshal(map[string]any{"@type":"annot","note": a.Note}) }
func (t Timestamp) MarshalJSON() ([]byte, error) { return json.Marshal(map[string]any{"@type":"ts","value": t.RFC3339}) }
func (d Date) MarshalJSON() ([]byte, error)      { return json.Marshal(map[string]any{"@type":"date","value": d.YYYYMMDD}) }
func (t Time) MarshalJSON() ([]byte, error)      { return json.Marshal(map[string]any{"@type":"time","value": t.HHMMSS}) }
