package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chandan-cmd-dev/jolt-go/jolt"
	"github.com/chandan-cmd-dev/jolt-go/joltsec"
)

// media types
const (
	mtJOLT = "application/jolt"
	mtJB   = "application/jolt-binary"
	mtJSEC = "application/jolt-sec"
	mtJSON = "application/json"
)

var (
	kr  joltsec.Keyring // nil unless -keyfile is provided
	kid = "k1"
	alg = joltsec.AlgXChaCha20Poly1305
)

func main() {
	var keyfile string
	var addr string
	flag.StringVar(&keyfile, "keyfile", "", "32-byte symmetric key to enable JOLT-SEC")
	flag.StringVar(&addr, "listen", ":8081", "listen address")
	flag.Parse()

	if keyfile != "" {
		key, err := os.ReadFile(keyfile)
		if err != nil {
			log.Fatalf("read keyfile: %v", err)
		}
		// trim trailing newlines if any
		for len(key) > 0 && (key[len(key)-1] == '\n' || key[len(key)-1] == '\r') {
			key = key[:len(key)-1]
		}
		kr = joltsec.StaticKeyring{kid: key}
		log.Printf("JOLT-SEC enabled (kid=%s)", kid)
	} else {
		log.Printf("JOLT-SEC disabled (start with -keyfile to enable encrypted responses)")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/echo", handleEcho)           // POST echo (JSON/JOLT/JOLT-SEC in; negotiated out)
	mux.HandleFunc("/joltsec/decrypt", handleDec) // POST encrypted -> JSON (useful for quick tests)
	log.Printf("mini REST API on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleEcho(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read: "+err.Error(), 400)
		return
	}

	// Accept JSON, JOLT, or JOLT-SEC in the request
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	var obj any

	switch {
	case strings.HasPrefix(ct, mtJSON):
		// accept JSON with // and /* */ comments
		if err := jolt.UnmarshalJSONWithComments(body, &obj); err != nil {
			http.Error(w, "json decode: "+err.Error(), 400)
			return
		}
	case strings.HasPrefix(ct, mtJOLT) || strings.HasPrefix(ct, mtJB):
		v, err := jolt.DecodeBinary(body)
		if err != nil {
			http.Error(w, "jolt decode: "+err.Error(), 400)
			return
		}
		obj = v
	case strings.HasPrefix(ct, mtJSEC):
		if kr == nil {
			http.Error(w, "jolt-sec disabled (start server with -keyfile)", 400)
			return
		}
		// no AAD check here (keep the demo simple)
		v, _, err := joltsec.DecryptJOLT(body, kr)
		if err != nil {
			http.Error(w, "jolt-sec decrypt: "+err.Error(), 400)
			return
		}
		obj = v
	default:
		http.Error(w, "unsupported Content-Type", 415)
		return
	}

	// Echo back with simple metadata
	resp := map[string]any{
		"$meta": map[string]any{
			"type":      "urn:jolt:demo/Echo",
			"version":   "1.0.0",
			"received":  time.Now().UTC().Format(time.RFC3339Nano),
			"userAgent": r.UserAgent(),
		},
		"$body": obj,
	}

	// Negotiate output: prefer request Accept; default JSON
	accept := strings.ToLower(r.Header.Get("Accept"))
	switch {
	case strings.Contains(accept, mtJSEC):
		if kr == nil {
			http.Error(w, "jolt-sec disabled", 406)
			return
		}
		hdr := joltsec.Header{Alg: alg, KeyID: kid, Extra: map[string]string{
			"m": r.Method, "p": r.URL.Path,
		}}
		sec, err := joltsec.EncryptJOLT(resp, hdr, kr)
		if err != nil {
			http.Error(w, "encrypt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", mtJSEC)
		w.WriteHeader(200)
		w.Write(sec)
		return

	case strings.Contains(accept, mtJOLT) || strings.Contains(accept, mtJB):
		jb, err := jolt.EncodeBinary(resp)
		if err != nil {
			http.Error(w, "encode jolt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", mtJOLT) // use mtJB if you prefer
		w.WriteHeader(200)
		w.Write(jb)
		return

	default:
		js, _ := jolt.MarshalJSONCompat(resp, true)
		w.Header().Set("Content-Type", mtJSON)
		w.WriteHeader(200)
		w.Write(js)
	}
}

func handleDec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	if kr == nil {
		http.Error(w, "jolt-sec disabled (start server with -keyfile)", 400)
		return
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	v, _, err := joltsec.DecryptJOLT(body, kr)
	if err != nil {
		http.Error(w, "decrypt: "+err.Error(), 400)
		return
	}
	js, _ := jolt.MarshalJSONCompat(v, true)
	w.Header().Set("Content-Type", mtJSON)
	w.Write(js)
}

// -------- helpers for generic JSON ("string-interface") --------

type AnyMap = map[string]any

// decodeJSON keeps arbitrary keys/values (string-interface pattern)
func decodeJSON(raw []byte) (AnyMap, error) {
	var v AnyMap
	err := json.Unmarshal(raw, &v)
	return v, err
}

// encodeJSON pretty-prints a generic map
func encodeJSON(v AnyMap) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

func _unused(_ ...any) {} // placate linters in minimal demo
