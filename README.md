# JOLT & JOLT‑SEC — Developer Guide

This guide explains **what JOLT is**, why it exists, how to **use it in Go**, how to **integrate with REST APIs**, and how to **adopt it alongside JSON** without breaking existing clients. It also covers **JOLT‑SEC** for authenticated encryption and includes a practical threat model, key management guidance, and lots of copy‑pasteable examples.

---

## 1) What is JOLT?

**JOLT** is a compact, self‑describing **binary** serialization format with a JSON‑like envelope for evolution:

- **Envelope**: 
  - `$meta`: `{ type, version, createdAt, ... }` — identifies the domain type and schema version
  - `$body`: the actual data (objects, arrays, or typed values)
- **Rich types** via `@type`: `int` (arbitrary‑precision), `dec` (decimal math), `ts/date/time`, `uuid`, `bin`, `set`, `map`, `link`, `annot`.
- **Comment support**: accepts `//` and `/* ... */` when reading JSON; `$comment` fields can be preserved if enabled.
- **Canonical binary**: stable bytes → great for hashing, caching, ETags, signatures.
- **Lossless round‑trip**: JSON ⇆ JOLT without float/number surprises.

**JOLT‑SEC** wraps JOLT with **AEAD** (XChaCha20‑Poly1305 or AES‑256‑GCM) for authenticated, confidential payloads. Optional **AAD** can bind method/path/tenant to stop replay or cross‑endpoint use.

**Media types**:
- `application/jolt` (alias: `application/jolt-binary`)
- `application/jolt-sec` (encrypted)
- `application/json` (fallback/interop)

---

## 2) How JOLT compares (at a glance)

| Capability / Format              | JSON | CBOR | Protobuf | Avro | MsgPack | **JOLT** |
|----------------------------------|:----:|:----:|:--------:|:----:|:------:|:--------:|
| Human‑readable (decoded)         | ✅   | ⚠️*  | ❌       | ❌   | ⚠️*    | ✅       |
| Binary compactness               | ❌   | ✅   | ✅       | ✅   | ✅     | ✅       |
| Self‑describing default          | ❌   | ⚠️   | ❌       | ⚠️   | ❌     | ✅       |
| Precise `dec` / big `int`        | ❌   | ⚠️   | ⚠️       | ✅   | ❌     | ✅       |
| Built‑in versioning metadata     | ❌   | ❌   | ❌       | ⚠️   | ❌     | ✅       |
| Comment-friendly during dev      | ❌   | ❌   | ❌       | ❌   | ❌     | ✅       |
| App‑layer encryption (standard)  | ❌   | ❌   | ❌       | ❌   | ❌     | ✅ (JOLT‑SEC) |
| Easy content negotiation (REST)  | ✅   | ✅   | ⚠️       | ⚠️   | ✅     | ✅       |

\* CBOR/MsgPack decode to readable JSON, but their default use often relies on out‑of‑band tagging. **JOLT ships self‑description and versioning in the payload envelope** so it works across teams/services without brittle side contracts.

---

## 3) Adoption strategy (JSON at the edge, JOLT inside)

Most teams don’t flip a switch in one day. Use this hybrid approach:

1. **Ingress** (clients → API): accept **JSON**, **JOLT**, and **JOLT‑SEC**.
2. **Internal**: immediately convert to **canonical JOLT‑B** and use it for storage, queues, cache keys, and service‑to‑service hops.
3. **Egress** (API → clients): honor `Accept` header (prefer JOLT/JOLT‑SEC for capable clients; fallback to JSON).

Benefits:
- Dev‑friendly for external users (JSON); consistent, typed internals (JOLT).
- Stable ETags and signatures on the canonical bytes.
- Easy migration: add JOLT without breaking existing JSON clients.

---

## 4) Go usage

### Install
```bash
go get github.com/chandan-cmd-dev/jolt-go
```

### Encode / decode
```go
import "github.com/chandan-cmd-dev/jolt-go/jolt"

// Any Go value (map[string]any, structs, slices…)
blob, err := jolt.EncodeBinary(v)  // to canonical JOLT‑B
out,  err := jolt.DecodeBinary(blob)
```

### JSON ⇆ JOLT (with comments)
```go
var v any
_ = jolt.UnmarshalJSONWithComments(srcJSON, &v) // supports // and /* */

jolt.PreserveComments = true
defer func(){ jolt.PreserveComments = false }()

bin, _ := jolt.EncodeBinary(v)
round, _ := jolt.DecodeBinary(bin)
js,   _ := jolt.MarshalJSONCompat(round, true) // pretty JSON from JOLT values
```

### Rich types (examples)
```json
{ "@type":"int",  "value":"9223372036854775808" }
{ "@type":"dec",  "value":"1999.95" }
{ "@type":"ts",   "value":"2025-08-08T10:00:00Z" }
{ "@type":"uuid", "value":"73bca6bf-8d9d-4095-93f4-13e85485f2db" }
{ "@type":"bin",  "value":"AAECAwQ=" }
{ "@type":"set",  "value":["a","b","c"] }
{ "@type":"map",  "value":[
  { "key": { "@type":"uuid","value":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" }, "value": 1 },
  { "key": { "@type":"uuid","value":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" }, "value": 2 }
] }
```

> Plain JSON numbers are coerced: integers → **`int`**; non‑integers → **`dec`** for precision.

---

## 5) REST API: accept JSON/JOLT/JOLT‑SEC; respond by `Accept`

Minimal handler:
```go
func echo(w http.ResponseWriter, r *http.Request) {
  body, _ := io.ReadAll(r.Body)
  defer r.Body.Close()

  var obj any
  ct := strings.ToLower(r.Header.Get("Content-Type"))
  switch {
  case strings.HasPrefix(ct, "application/json"):
    _ = jolt.UnmarshalJSONWithComments(body, &obj)
  case strings.HasPrefix(ct, "application/jolt") ||
       strings.HasPrefix(ct, "application/jolt-binary"):
    var err error; obj, err = jolt.DecodeBinary(body); if err != nil { http.Error(w, err.Error(), 400); return }
  case strings.HasPrefix(ct, "application/jolt-sec"):
    v, _, err := joltsec.DecryptJOLT(body, keyring); if err != nil { http.Error(w, err.Error(), 400); return }
    obj = v
  default:
    http.Error(w, "415 unsupported media type", 415); return
  }

  resp := map[string]any{ "$meta": map[string]any{"type":"urn:jolt:Echo","version":"1"}, "$body": obj }

  acc := strings.ToLower(r.Header.Get("Accept"))
  switch {
  case strings.Contains(acc, "application/jolt-sec"):
    sec, _ := joltsec.EncryptJOLT(resp, joltsec.Header{Alg:joltsec.AlgXChaCha20Poly1305, KeyID:"k1"}, keyring)
    w.Header().Set("Content-Type", "application/jolt-sec"); w.Write(sec)
  case strings.Contains(acc, "application/jolt"),
       strings.Contains(acc, "application/jolt-binary"):
    jb, _ := jolt.EncodeBinary(resp)
    w.Header().Set("Content-Type", "application/jolt"); w.Write(jb)
  default:
    js, _ := jolt.MarshalJSONCompat(resp, true)
    w.Header().Set("Content-Type", "application/json"); w.Write(js)
  }
}
```

**Inline curl** (no temp files):

- JSON → **JOLT** response
```bash
curl -sS -X POST http://localhost:8080/echo   -H 'Content-Type: application/json'   -H 'Accept: application/jolt'   --data '{"$body":{"$id":"o1001","qty":2,"price":{"@type":"dec","value":"19.99"}}}'   -o /tmp/resp.jb && go run ./cmd/jolt -mode decode < /tmp/resp.jb | jq
```

- JSON → **JOLT‑SEC** response
```bash
curl -sS -X POST http://localhost:8080/echo   -H 'Content-Type: application/json'   -H 'Accept: application/jolt-sec'   --data '{"$body":{"$id":"secure-1","pii":{"email":"a@b.com"}}}'   -o /tmp/resp.sec && go run ./cmd/joltsec -mode decrypt -keyfile /tmp/jolt.key < /tmp/resp.sec | jq
```

- Fully **encrypted request** (process substitution)
```bash
curl -sS -X POST http://localhost:8080/echo   -H 'Content-Type: application/jolt-sec'   -H 'Accept: application/json'   --data-binary @<( echo '{"$body":{"$id":"enc-99","ok":true}}'     | go run ./cmd/joltsec -mode encrypt -keyfile /tmp/jolt.key -alg xchacha -aad-method POST -aad-path /echo )   | jq
```

---

## 6) Storage, queues, and caching with JOLT

**Always store canonical JOLT‑B**. Reasons: smaller, precise types, stable bytes for hashing/ETags.

Example DAO pattern:
```go
jb, _ := jolt.EncodeBinary(v)   // convert JSON/JOLT to JOLT‑B once
// store jb in a BYTEA/BLOB column; optionally store $meta as JSON for indexing
```

**Queues/events**: publish JOLT‑B, not JSON:
```go
evt := map[string]any{ "$meta":{"type":"urn:jolt:event/OrderCreated","version":"1"}, "$body": v }
payload, _ := jolt.EncodeBinary(evt)
// write payload to Kafka/NATS/etc
```

**ETags**:
```go
sum := sha256.Sum256(jb)
etag := `W/"` + hex.EncodeToString(sum[:8]) + `"`
w.Header().Set("ETag", etag)
```

---

## 7) JOLT‑SEC (encryption) in practice

### Library
```go
import "github.com/chandan-cmd-dev/jolt-go/joltsec"

key := make([]byte, 32)            // fetch from KMS/HSM in prod
for i := range key { key[i] = byte(i) }
kr := joltsec.StaticKeyring{"k1": key}

hdr := joltsec.Header{
  Alg:   joltsec.AlgXChaCha20Poly1305, // or AES‑256‑GCM
  KeyID: "k1",
  Extra: map[string]string{"m":"POST","p":"/orders"}, // AAD binding (optional)
}

sec, _ := joltsec.EncryptJOLT(v, hdr, kr)
out, hdr2, _ := joltsec.DecryptJOLT(sec, kr)
```

### Operational guidance
- Keep **TLS**; JOLT‑SEC complements it (payload security & replay protection).
- Use **AAD** to bind **method + path** (and optionally tenant, request ID).
- Rotate 256‑bit keys every 60–90 days and on compromise.
- Use KMS/HSM (AWS KMS, GCP KMS, Vault) for key storage.

---

## 8) Complex nested example (all types)

```jsonc
{
  "$meta": {
    "type": "urn:jolt:example/Invoice",
    "version": "3.0.0",
    "createdAt": { "@type": "ts", "value": "2025-08-08T10:00:00Z" },
    "$comment": "All key types, deep nesting, and annotations"
  },
  "$body": {
    "$id": "inv:2025-00042",
    "account": {
      "id":    { "@type": "uuid", "value": "1f0b9aaf-0d1e-4f0d-a32f-cc7e4d7c4e76" },
      "email": "buyer@example.com"
    },
    "lines": [
      { "sku":"SKU-001", "qty":{ "@type":"int","value":"3" }, "price":{ "@type":"annot","label":"USD","value":{ "@type":"dec","value":"19.99" } }, "tags":["new","promo"] },
      { "sku":"SKU-002", "qty":{ "@type":"int","value":"1" }, "price":{ "@type":"annot","label":"USD","value":{ "@type":"dec","value":"249.00" } },
        "bundle": { "items":[ { "name":"Case","price":{ "@type":"dec","value":"9.95" } }, { "name":"Cable","price":{ "@type":"dec","value":"5.50" } } ] } }
    ],
    "totals": { "net":{ "@type":"dec","value":"308.92" }, "tax":{ "@type":"dec","value":"37.07" }, "gross":{ "@type":"dec","value":"345.99" } },
    "issued":  { "@type": "date", "value": "2025-08-08" },
    "cutoff":  { "@type": "time", "value": "17:30:00" },
    "flags":   { "@type": "set",  "value": ["export","giftwrap","fragile"] },
    "doc":     { "@type": "link", "value": "https://example.com/invoice/2025-00042" },
    "payload": { "@type": "bin",  "value": "AAECAwQFBgcICQ==" },
    "attrs":   { "@type": "map",  "value": [
      { "key":{ "@type":"uuid","value":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" }, "value":"alpha" },
      { "key":{ "@type":"uuid","value":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" }, "value":"beta"  }
    ] },
    "$comment": "JOLT supports deep nesting and preserves annotations."
  }
}
```

Round‑trip:
```bash
go run ./cmd/jolt -mode encode < sample.json > /tmp/sample.jb
go run ./cmd/jolt -mode decode < /tmp/sample.jb | jq
```

Encrypted:
```bash
go run ./cmd/joltsec -mode encrypt -keyfile /tmp/jolt.key -alg xchacha   -aad-method POST -aad-path /invoices   < sample.json > /tmp/sample.sec
go run ./cmd/joltsec -mode decrypt -keyfile /tmp/jolt.key < /tmp/sample.sec | jq
```

---

## 9) FAQ (dev‑oriented)

- **Do I have to switch all clients to JOLT?** No. Accept JSON + JOLT; store canonical JOLT‑B; respond by `Accept`.
- **Will JOLT handle complex nested structures?** Yes — objects/arrays/maps/sets/annotated values of arbitrary depth.
- **How do I keep exact decimals?** Use `@type:"dec"` — no float rounding.
- **How do I secure payloads?** Use **JOLT‑SEC** with AEAD; bind AAD to method/path; keep TLS.

---

## License
Apache‑2.0 (or your preferred license)
