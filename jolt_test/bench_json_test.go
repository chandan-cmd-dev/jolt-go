package jolt_test

import (
	"encoding/json"
	"testing"
)

// Uses orderJSON() helper from testhelpers_test.go

func BenchmarkJSONDecode(b *testing.B) {
	raw := orderJSON()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	// Parse once, then measure encoding throughput
	var v any
	if err := json.Unmarshal(orderJSON(), &v); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONRoundTrip(b *testing.B) {
	raw := orderJSON()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			b.Fatal(err)
		}
		if _, err := json.Marshal(v); err != nil {
			b.Fatal(err)
		}
	}
}
