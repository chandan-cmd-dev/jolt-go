package jolt_test

import (
  "os"
  "path/filepath"

  "github.com/yourname/jolt-go/jolt"
)

const orderJSONFallback = `{
  "$meta": { "type": "urn:jolt:example/Order", "version": "2.1.0",
             "createdAt": { "@type":"ts","value":"2025-08-08T07:42:01.344243Z" } },
  "$body": {
    "$id": "order:9f2e",
    "number": "SO-12988",
    "price": { "@type": "dec", "value": "1999.95" },
    "qty":    { "@type": "int", "value": "2" },
    "tags": ["gift","festival"],
    "uuid": { "@type": "uuid", "value": "73bca6bf-8d9d-4095-93f4-13e85485f2db" }
  }
}`

func orderJSON() []byte {
  path := filepath.Join("testdata", "order.json")
  if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
    return b
  }
  _ = os.MkdirAll(filepath.Dir(path), 0o755)
  _ = os.WriteFile(path, []byte(orderJSONFallback), 0o644)
  return []byte(orderJSONFallback)
}

func mustDec(s string) jolt.Decimal {
  d, _ := jolt.DecFromString(s)
  return d
}
