package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chandan-cmd-dev/jolt-go/jolt"
	"github.com/chandan-cmd-dev/jolt-go/joltsec"
)

// In-memory "DB": we store canonical JOLT-B bytes for responses.
type store struct {
	mu  sync.RWMutex
	m   map[string][]byte // id -> JOLT-B
	met map[string]jolt.Meta
}

func newStore() *store { return &store{m: map[string][]byte{}, met: map[string]jolt.Meta{}} }

func (s *store) put(id string, meta jolt.Meta, jb []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = jb
	s.met[id] = meta
}

func (s *store) get(id string) (meta jolt.Meta, jb []byte, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jb, ok = s.m[id]
	if !ok {
		return jolt.Meta{}, nil, false
	}
	return s.met[id], jb, true
}

var (
	// Keyring for JOLT-SEC (optional). If nil, /joltsec is disabled.
	kr  joltsec.Keyring
	alg joltsec.Alg
	kid string
)

func main() {
	var keyfile string
	var algFlag string
	flag.StringVar(&keyfile, "keyfile", "", "path to 32-byte symmetric key to enable JOLT-SEC")
	flag.StringVar(&algFlag, "alg", "xchacha", "xchacha | aesgcm (for JOLT-SEC)")
	flag.Parse()

	switch strings.ToLower(algFlag) {
	case "xchacha", "xchacha20", "xchacha20poly1305":
		alg = joltsec.AlgXChaCha20Poly1305
	case "aes", "aesgcm", "aes-256-gcm":
		alg = joltsec.AlgAES256GCM
	default:
		log.Fatalf("unknown -alg: %s", algFlag)
	}
	kid = "k1" // demo key id

	if keyfile != "" {
		key, err := os.ReadFile(keyfile)
		if err != nil {
			log.Fatalf("read keyfile: %v", err)
		}
		// Trim trailing newlines if someone used base64 output or echo
		for len(key) > 0 && (key[len(key)-1] == '\n' || key[len(key)-1] == '\r') {
			key = key[:len(key)-1]
		}
		kr = joltsec.StaticKeyring{kid: key}
		log.Printf("JOLT-SEC enabled (%s, kid=%s)", alg, kid)
	} else {
		log.Printf("JOLT-SEC disabled (start with -keyfile to enable /orders in application/jolt-sec)")
	}

	s := newStore()

	mux := http.NewServeMux()
	// POST /orders — create an order in whichever format Content-Type indicates
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleCreate(s, w, r)
	})
	// GET /orders/{id} — return the order in a negotiated format
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/orders/")
		handleGet(s, w, r, id)
	})

	addr := ":8080"
	log.Printf("REST API on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleCreate(s *store, w http.ResponseWriter, r *http.Request) {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), 400)
		return
	}
	defer r.Body.Close()

	var v any
	var meta jolt.Meta
	switch {
	case strings.HasPrefix(ct, "application/json"):
		// allow // and /* */ comments too
		if err := jolt.UnmarshalJSONWithComments(body, &v); err != nil {
			http.Error(w, "json decode: "+err.Error(), 400)
			return
		}
		// Try to extract envelope meta if present
		if m, ok := v.(map[string]any); ok {
			if mm, ok := m["$meta"].(map[string]any); ok {
				meta = metaFromMap(mm)
			}
		}

	case strings.HasPrefix(ct, "application/jolt") || strings.HasPrefix(ct, "application/jolt-binary"):
		dec, err := jolt.DecodeBinary(body)
		if err != nil {
			http.Error(w, "jolt decode: "+err.Error(), 400)
			return
		}
		v = dec
		if env, ok := v.(jolt.Envelope); ok {
			meta = env.Meta
		} else if m, ok := v.(map[string]any); ok {
			if mm, ok := m["$meta"].(map[string]any); ok {
				meta = metaFromMap(mm)
			}
		}

	case strings.HasPrefix(ct, "application/jolt-sec"):
		if kr == nil {
			http.Error(w, "jolt-sec disabled; start server with -keyfile", 400)
			return
		}
		dec, hdr, err := joltsec.DecryptJOLT(body, kr)
		if err != nil {
			http.Error(w, "jolt-sec decrypt: "+err.Error(), 400)
			return
		}
		// AAD binding: check method/path in header.Extra (if client supplied)
		if !aadMatches(hdr.Extra, r.Method, r.URL.Path) {
			http.Error(w, "aad mismatch", 400)
			return
		}
		v = dec
		if env, ok := v.(jolt.Envelope); ok {
			meta = env.Meta
		} else if m, ok := v.(map[string]any); ok {
			if mm, ok := m["$meta"].(map[string]any); ok {
				meta = metaFromMap(mm)
			}
		}

	default:
		http.Error(w, "unsupported Content-Type", 415)
		return
	}

	// Assign ID
	id := extractID(v)
	if id == "" {
		id = fmt.Sprintf("auto:%d", time.Now().UnixNano())
	}

	// Canonicalize to JOLT-B for storage
	jb, err := jolt.EncodeBinary(v)
	if err != nil {
		http.Error(w, "encode to jolt: "+err.Error(), 500)
		return
	}
	s.put(id, meta, jb)

	// Negotiate response format
	accept := strings.ToLower(r.Header.Get("Accept"))
	mt := negotiate(accept, "application/jolt-sec", "application/jolt", "application/jolt-binary", "application/json")

	switch mt {
	case "application/jolt-sec":
		if kr == nil {
			http.Error(w, "jolt-sec not enabled", 406)
			return
		}
		// Build envelope/header for response
		var obj any
		if err := jolt.UnmarshalJSONWithComments(mustDecodeJSON(jb), &obj); err != nil {
			// If decode to JSON fails (it shouldn't), fallback to decoding JOLT and using that
			obj, _ = jolt.DecodeBinary(jb)
		}
		hdr := joltsec.Header{
			Alg:   alg,
			KeyID: kid,
			Extra: buildAAD(r.Method, "/orders/"+id),
		}
		sec, err := joltsec.EncryptJOLT(obj, hdr, kr)
		if err != nil {
			http.Error(w, "encrypt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/jolt-sec")
		w.Header().Set("Location", "/orders/"+id)
		w.WriteHeader(http.StatusCreated)
		w.Write(sec)

	case "application/jolt", "application/jolt-binary":
		w.Header().Set("Content-Type", mt) // echo the client's chosen jolt media type
		w.Header().Set("Location", "/orders/"+id)
		w.WriteHeader(http.StatusCreated)
		w.Write(jb)

	default: // JSON
		js, _ := jolt.MarshalJSONCompat(mustDecodeJOLT(jb), true)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/orders/"+id)
		w.WriteHeader(http.StatusCreated)
		w.Write(js)
	}
}

func handleGet(s *store, w http.ResponseWriter, r *http.Request, id string) {
	meta, jb, ok := s.get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = meta // could be used to gate versions

	accept := strings.ToLower(r.Header.Get("Accept"))
	mt := negotiate(accept, "application/jolt-sec", "application/jolt", "application/jolt-binary", "application/json")

	switch mt {
	case "application/jolt-sec":
		if kr == nil {
			http.Error(w, "jolt-sec not enabled", 406)
			return
		}
		var obj any
		if err := jolt.UnmarshalJSONWithComments(mustDecodeJSON(jb), &obj); err != nil {
			obj, _ = jolt.DecodeBinary(jb)
		}
		hdr := joltsec.Header{
			Alg:   alg,
			KeyID: kid,
			Extra: buildAAD(http.MethodGet, "/orders/"+id),
		}
		sec, err := joltsec.EncryptJOLT(obj, hdr, kr)
		if err != nil {
			http.Error(w, "encrypt: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/jolt-sec")
		w.Write(sec)

	case "application/jolt", "application/jolt-binary":
		w.Header().Set("Content-Type", mt)
		w.Write(jb)

	default:
		js, _ := jolt.MarshalJSONCompat(mustDecodeJOLT(jb), true)
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

func negotiate(accept string, supported ...string) string {
	if accept == "" {
		return supported[len(supported)-1] // default to last (json)
	}
	for _, s := range supported {
		if strings.Contains(accept, s) {
			return s
		}
	}
	return supported[len(supported)-1]
}

func extractID(v any) string {
	// Try $body.$id or top-level $id
	if env, ok := v.(jolt.Envelope); ok {
		if m, ok := env.Body.(map[string]any); ok {
			if id, ok := m["$id"].(string); ok {
				return id
			}
		}
	}
	if m, ok := v.(map[string]any); ok {
		if b, ok := m["$body"].(map[string]any); ok {
			if id, ok := b["$id"].(string); ok {
				return id
			}
		}
		if id, ok := m["$id"].(string); ok {
			return id
		}
	}
	return ""
}

func metaFromMap(mm map[string]any) jolt.Meta {
	var meta jolt.Meta
	if v, ok := mm["type"].(string); ok {
		meta.Type = v
	}
	if v, ok := mm["version"].(string); ok {
		meta.Version = v
	}
	// createdAt optional
	return meta
}

func mustDecodeJOLT(jb []byte) any {
	v, _ := jolt.DecodeBinary(jb)
	return v
}

func mustDecodeJSON(jb []byte) []byte {
	// Decode to any then marshal JSON (lossless for our supported types)
	v, _ := jolt.DecodeBinary(jb)
	js, _ := jolt.MarshalJSONCompat(v, true)
	return js
}

func buildAAD(method, path string) map[string]string {
	return map[string]string{
		"m":  method,
		"p":  path,
		"ts": time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func aadMatches(extra map[string]string, method, path string) bool {
	if extra == nil {
		return true // if client didn’t send AAD, don’t enforce
	}
	return extra["m"] == method && extra["p"] == path
}

// (unused here, but handy for stdin reads if you embed tools later)
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
