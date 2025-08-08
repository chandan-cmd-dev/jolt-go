package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yourname/jolt-go/jolt"
	"github.com/yourname/jolt-go/joltsec"
)

func main() {
	var keyfile string
	flag.StringVar(&keyfile, "keyfile", "", "path to 32-byte symmetric key (required for /joltsec)")
	flag.Parse()

	var kr joltsec.Keyring
	if keyfile != "" {
		key, err := os.ReadFile(keyfile)
		if err != nil {
			log.Fatalf("read keyfile: %v", err)
		}
		kr = joltsec.StaticKeyring{"k1": key}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/json", handleJSON)
	mux.HandleFunc("/jolt", handleJOLT)
	mux.HandleFunc("/joltsec", func(w http.ResponseWriter, r *http.Request) {
		handleJOLTSEC(w, r, kr)
	})

	log.Println("bench server on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleJSON(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	startRead := time.Now()
	body, err := io.ReadAll(r.Body)
	readDur := time.Since(startRead)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	startDec := time.Now()
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		http.Error(w, "json decode: "+err.Error(), 400)
		return
	}
	decDur := time.Since(startDec)

	report(w, "json", len(body), readDur, decDur, 0)
}

func handleJOLT(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	startRead := time.Now()
	body, err := io.ReadAll(r.Body)
	readDur := time.Since(startRead)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	startDec := time.Now()
	_, err = jolt.DecodeBinary(body)
	decDur := time.Since(startDec)
	if err != nil {
		http.Error(w, "jolt decode: "+err.Error(), 400)
		return
	}

	report(w, "jolt", len(body), readDur, decDur, 0)
}

func handleJOLTSEC(w http.ResponseWriter, r *http.Request, kr joltsec.Keyring) {
	defer r.Body.Close()
	startRead := time.Now()
	body, err := io.ReadAll(r.Body)
	readDur := time.Since(startRead)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if kr == nil {
		http.Error(w, "no key configured; start server with -keyfile", 500)
		return
	}

	startDec := time.Now()
	_, _, err = joltsec.DecryptJOLT(body, kr) // expects kid "k1" in header
	decDur := time.Since(startDec)
	if err != nil {
		http.Error(w, "jolt-sec decrypt: "+err.Error(), 400)
		return
	}

	report(w, "jolt-sec", len(body), readDur, decDur, 0)
}

func report(w http.ResponseWriter, kind string, bytes int, readDur, decDur, extra time.Duration) {
	js := fmt.Sprintf(`{
  "kind": %q,
  "bytes": %d,
  "read_ms": %.3f,
  "decode_ms": %.3f,
  "extra_ms": %.3f
}
`, kind, bytes, float64(readDur.Microseconds())/1000, float64(decDur.Microseconds())/1000, float64(extra.Microseconds())/1000)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(js))
}
