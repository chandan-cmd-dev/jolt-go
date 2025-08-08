package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yourname/jolt-go/jolt"
	"github.com/yourname/jolt-go/joltsec"
)

func main() {
	var mode string
	var keyfile string
	var alg string
	var kid string
	var indent bool
	var aadMethod string
	var aadPath string

	flag.StringVar(&aadMethod, "aad-method", "", "HTTP method to bind in AAD (e.g. POST)")
	flag.StringVar(&aadPath, "aad-path", "", "HTTP path to bind in AAD (e.g. /orders)")
	flag.StringVar(&mode, "mode", "", "encrypt | decrypt")
	flag.StringVar(&keyfile, "keyfile", "", "path to 32-byte key file (required)")
	flag.StringVar(&alg, "alg", "xchacha", "xchacha | aesgcm")
	flag.StringVar(&kid, "kid", "k1", "key id to embed in header")
	flag.BoolVar(&indent, "indent", true, "pretty-print JSON on decrypt")
	flag.Parse()

	if mode != "encrypt" && mode != "decrypt" {
		fatalf("invalid -mode (use encrypt or decrypt)")
	}
	if keyfile == "" {
		fatalf("missing -keyfile")
	}

	key, err := os.ReadFile(keyfile)
	if err != nil {
		fatalf("read key: %v", err)
	}
	key = trimNewlines(key) // allow base64-free raw files created with `openssl rand -out`

	var suite joltsec.Alg
	switch strings.ToLower(alg) {
	case "xchacha", "xchacha20", "xchacha20poly1305":
		suite = joltsec.AlgXChaCha20Poly1305
	case "aes", "aesgcm", "aes-256-gcm":
		suite = joltsec.AlgAES256GCM
	default:
		fatalf("unknown -alg: %s", alg)
	}

	switch mode {
	case "encrypt":
		data, err := ioReadAll(os.Stdin)
		if err != nil {
			fatalf("read stdin: %v", err)
		}
		var v any
		if err := jolt.UnmarshalJSONWithComments(data, &v); err != nil {
			fatalf("parse json: %v", err)
		}
		kr := joltsec.StaticKeyring{kid: key}
		extra := map[string]string{"tool": "joltsec"}
		if aadMethod != "" {
			extra["m"] = aadMethod
		}
		if aadPath != "" {
			extra["p"] = aadPath
		}

		hdr := joltsec.Header{
			Alg:   suite,
			KeyID: kid,
			Extra: extra,
		}
		jsec, err := joltsec.EncryptJOLT(v, hdr, kr)
		if err != nil {
			fatalf("encrypt: %v", err)
		}
		os.Stdout.Write(jsec)

	case "decrypt":
		jsec, err := ioReadAll(os.Stdin)
		if err != nil {
			fatalf("read stdin: %v", err)
		}
		kr := joltsec.StaticKeyring{kid: key}
		v, hdr, err := joltsec.DecryptJOLT(jsec, kr)
		if err != nil {
			fatalf("decrypt: %v", err)
		}
		_ = hdr // available if you want to print header metadata later

		js, err := jolt.MarshalJSONCompat(v, indent)
		if err != nil {
			fatalf("marshal json: %v", err)
		}
		os.Stdout.Write(js)
	}
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+f+"\n", a...)
	os.Exit(2)
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

func trimNewlines(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
