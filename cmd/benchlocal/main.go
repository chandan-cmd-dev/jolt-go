package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/yourname/jolt-go/jolt"
)

func main() {
	input, err := os.ReadFile("jolt_test/testdata/order.json")
	if err != nil {
		panic(err)
	}

	// JSON decode
	var v any
	t0 := time.Now()
	if err := json.Unmarshal(input, &v); err != nil {
		panic(err)
	}
	jsonDec := time.Since(t0)

	// JOLT encode
	t1 := time.Now()
	jb, err := jolt.EncodeBinary(v)
	if err != nil {
		panic(err)
	}
	joltEnc := time.Since(t1)

	// JOLT decode
	t2 := time.Now()
	_, err = jolt.DecodeBinary(jb)
	if err != nil {
		panic(err)
	}
	joltDec := time.Since(t2)

	// JSON re-encode (pretty to be fair about human-readability, but you can do compact)
	t3 := time.Now()
	_, _ = json.Marshal(v)
	jsonEnc := time.Since(t3)

	fmt.Printf("Sizes:\n  JSON: %d B\n  JOLT: %d B\n\n", len(input), len(jb))
	fmt.Printf("Timings (ms):\n  json_decode: %.3f\n  json_encode: %.3f\n  jolt_encode: %.3f\n  jolt_decode: %.3f\n",
		ms(jsonDec), ms(jsonEnc), ms(joltEnc), ms(joltDec))
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
