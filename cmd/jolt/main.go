package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/yourname/jolt-go/jolt"
)

func main() {
	var mode string
	flag.StringVar(&mode, "mode", "example", "example|encode|decode")
	flag.Parse()

	switch mode {
	case "example":
		env := jolt.Envelope{
			Meta: jolt.Meta{Type: "urn:jolt:example/Order", Version: "2.1.0", Created: jolt.Ptr(jolt.TSNowUTC())},
			Body: map[string]any{"number": "SO-12988", "qty": jolt.BigInt(2), "price": mustDec("1999.95")},
		}
		js, _ := jolt.MarshalJSONCompat(env, true)
		fmt.Println(string(js))
		bin, _ := jolt.EncodeBinary(env)
		fmt.Println("JOLT-B size:", len(bin))
	case "encode":
		data, err := ioReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		var v any
		if err := jolt.UnmarshalJSONWithComments(data, &v); err != nil {
			panic(err)
		}
		b, err := jolt.EncodeBinary(v)
		if err != nil {
			panic(err)
		}
		os.Stdout.Write(b)
	case "decode":
		b, err := ioReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		v, err := jolt.DecodeBinary(b)
		if err != nil {
			panic(err)
		}
		js, _ := jolt.MarshalJSONCompat(v, true)
		os.Stdout.Write(js)
	default:
		fmt.Println("unknown mode")
	}
}

func ioReadAll(f *os.File) ([]byte, error) {
	st, _ := f.Stat()
	if st.Size() > 0 {
		b := make([]byte, st.Size())
		n, err := f.Read(b)
		return b[:n], err
	}
	var buf []byte
	tmp := make([]byte, 8192)
	r := bufio.NewReader(f)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			return buf, err
		}
	}
}

func mustDec(s string) jolt.Decimal { d, _ := jolt.DecFromString(s); return d }
