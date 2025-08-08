package jolt

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
)

const (
	tagNull  byte = 0x00
	tagF     byte = 0x01
	tagT     byte = 0x02
	tagInt   byte = 0x03
	tagDec   byte = 0x04
	tagStr   byte = 0x05
	tagBin   byte = 0x06
	tagArr   byte = 0x07
	tagObj   byte = 0x08
	tagTS    byte = 0x09
	tagDate  byte = 0x0A
	tagTime  byte = 0x0B
	tagSet   byte = 0x0C
	tagMap   byte = 0x0D
	tagUUID  byte = 0x0E
	tagLink  byte = 0x0F
	tagAnnot byte = 0x10
	tagEnv   byte = 0x11
)

type Limits struct{ MaxDepth, MaxBytes int }

var DefaultLimits = Limits{MaxDepth: 1024, MaxBytes: 64 << 20}

var (
	ErrUnknownTag  = fmt.Errorf("jolt: unknown tag")
	ErrBadEnvelope = fmt.Errorf("jolt: invalid envelope meta")
	ErrTooDeep     = fmt.Errorf("jolt: nesting depth exceeded")
)

// PreserveComments controls whether "$comment" keys are retained when encoding/decoding.
var PreserveComments = false

func putUvarint(w io.Writer, x uint64) error {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], x)
	_, err := w.Write(buf[:n])
	return err
}
func readUvarint(r io.ByteReader) (uint64, error) { return binary.ReadUvarint(r) }

func putZigZag(w io.Writer, i int64) error {
	x := uint64((i << 1) ^ (i >> 63))
	return putUvarint(w, x)
}
func readZigZag(r io.ByteReader) (int64, error) {
	u, err := readUvarint(r)
	if err != nil {
		return 0, err
	}
	return int64((u >> 1) ^ uint64((int64(u&1)<<63)>>63)), nil
}

func writeBytes(w io.Writer, b []byte) error {
	if err := putUvarint(w, uint64(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}
func writeString(w io.Writer, s string) error { return writeBytes(w, []byte(s)) }

func EncodeBinary(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeAny(&buf, v, 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeAny(w io.Writer, v any, depth int) error {
	if depth > DefaultLimits.MaxDepth {
		return ErrTooDeep
	}
	switch x := v.(type) {
	case nil:
		_, err := w.Write([]byte{tagNull})
		return err
	case bool:
		if x {
			_, err := w.Write([]byte{tagT})
			return err
		}
		_, err := w.Write([]byte{tagF})
		return err
	case string:
		if _, err := w.Write([]byte{tagStr}); err != nil {
			return err
		}
		return writeString(w, x)
	case float64:
		if math.Trunc(x) == x {
			// integer-valued -> encode as JOLT Int
			return encodeAny(w, BigInt(int64(x)), depth) // depth not incremented here
		}
		// decimal -> encode as JOLT Decimal
		s := strconv.FormatFloat(x, 'f', -1, 64)
		d, err := DecFromString(s)
		if err != nil {
			return err
		}
		return encodeAny(w, d, depth)

	case float32:
		xf := float64(x)
		if math.Trunc(xf) == xf {
			return encodeAny(w, BigInt(int64(xf)), depth)
		}
		s := strconv.FormatFloat(xf, 'f', -1, 64)
		d, err := DecFromString(s)
		if err != nil {
			return err
		}
		return encodeAny(w, d, depth)

	case int, int8, int16, int32, int64:
		return encodeAny(w, BigInt(reflect.ValueOf(x).Int()), depth)

	case uint, uint8, uint16, uint32, uint64:
		// clamp into signed when safe; otherwise fall back to decimal to avoid overflow
		u := reflect.ValueOf(x).Uint()
		if u <= math.MaxInt64 {
			return encodeAny(w, BigInt(int64(u)), depth)
		}
		d, err := DecFromString(strconv.FormatUint(u, 10))
		if err != nil {
			return err
		}
		return encodeAny(w, d, depth)
	case Int:
		if _, err := w.Write([]byte{tagInt}); err != nil {
			return err
		}
		sign := byte(0x00)
		if x.V.Sign() < 0 {
			sign = 0x01
		}
		mag := new(big.Int).Abs(x.V).Bytes()
		if err := putUvarint(w, uint64(len(mag)+1)); err != nil {
			return err
		}
		if _, err := w.Write([]byte{sign}); err != nil {
			return err
		}
		_, err := w.Write(mag)
		return err
	case Decimal:
		if _, err := w.Write([]byte{tagDec}); err != nil {
			return err
		}
		sign := byte(0x00)
		if x.D.Negative {
			sign = 0x01
		}
		if _, err := w.Write([]byte{sign}); err != nil {
			return err
		}
		if err := putZigZag(w, int64(x.D.Exponent)); err != nil {
			return err
		}
		coef := x.D.Coeff.Bytes()
		if err := writeBytes(w, coef); err != nil {
			return err
		}
		return nil
	case Binary:
		if _, err := w.Write([]byte{tagBin}); err != nil {
			return err
		}
		return writeBytes(w, []byte(x))
	case Timestamp:
		if _, err := w.Write([]byte{tagTS}); err != nil {
			return err
		}
		return writeString(w, x.RFC3339)
	case Date:
		if _, err := w.Write([]byte{tagDate}); err != nil {
			return err
		}
		return writeString(w, x.YYYYMMDD)
	case Time:
		if _, err := w.Write([]byte{tagTime}); err != nil {
			return err
		}
		return writeString(w, x.HHMMSS)
	case UUID:
		if _, err := w.Write([]byte{tagUUID}); err != nil {
			return err
		}
		_, err := w.Write(x[:])
		return err
	case Link:
		if _, err := w.Write([]byte{tagLink}); err != nil {
			return err
		}
		return writeString(w, x.Ref)
	case Annot:
		if _, err := w.Write([]byte{tagAnnot}); err != nil {
			return err
		}
		return writeString(w, x.Note)
	case []any:
		if _, err := w.Write([]byte{tagArr}); err != nil {
			return err
		}
		if err := putUvarint(w, uint64(len(x))); err != nil {
			return err
		}
		for _, it := range x {
			if err := encodeAny(w, it, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		obj := x
		if !PreserveComments {
			obj = make(map[string]any, len(x))
			for k, v := range x {
				if k == "$comment" {
					continue
				} // drop only when flag is false
				obj[k] = v
			}
		}
		if _, err := w.Write([]byte{tagObj}); err != nil {
			return err
		}
		ks := make([]string, 0, len(obj))
		for k := range obj {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		if err := putUvarint(w, uint64(len(ks))); err != nil {
			return err
		}
		for _, k := range ks {
			if err := writeString(w, k); err != nil {
				return err
			}
			if err := encodeAny(w, obj[k], depth+1); err != nil {
				return err
			}
		}
		return nil
	case Set:
		if _, err := w.Write([]byte{tagSet}); err != nil {
			return err
		}
		tmp := make([][]byte, 0, len(x))
		for _, it := range x {
			var b bytes.Buffer
			if err := encodeAny(&b, it, depth+1); err != nil {
				return err
			}
			tmp = append(tmp, b.Bytes())
		}
		sort.Slice(tmp, func(i, j int) bool { return bytes.Compare(tmp[i], tmp[j]) < 0 })
		if err := putUvarint(w, uint64(len(tmp))); err != nil {
			return err
		}
		for _, enc := range tmp {
			if _, err := w.Write(enc); err != nil {
				return err
			}
		}
		return nil
	case Map:
		if _, err := w.Write([]byte{tagMap}); err != nil {
			return err
		}
		type kv struct{ k, v []byte }
		kvs := make([]kv, 0, len(x))
		for k, v := range x {
			var kb, vb bytes.Buffer
			if err := encodeAny(&kb, k, depth+1); err != nil {
				return err
			}
			if err := encodeAny(&vb, v, depth+1); err != nil {
				return err
			}
			kvs = append(kvs, kv{kb.Bytes(), vb.Bytes()})
		}
		sort.Slice(kvs, func(i, j int) bool { return bytes.Compare(kvs[i].k, kvs[j].k) < 0 })
		if err := putUvarint(w, uint64(len(kvs))); err != nil {
			return err
		}
		for _, p := range kvs {
			if _, err := w.Write(p.k); err != nil {
				return err
			}
			if _, err := w.Write(p.v); err != nil {
				return err
			}
		}
		return nil
	case Envelope:
		if _, err := w.Write([]byte{tagEnv}); err != nil {
			return err
		}
		m := map[string]any{"type": x.Meta.Type, "schema": x.Meta.Schema, "version": x.Meta.Version, "features": x.Meta.Features}
		if x.Meta.Created != nil {
			m["createdAt"] = *x.Meta.Created
		}
		if x.Meta.Sig != nil {
			m["sig"] = x.Meta.Sig
		}
		if err := encodeAny(w, m, depth+1); err != nil {
			return err
		}
		return encodeAny(w, x.Body, depth+1)
	default:
		blob, err := json.Marshal(x)
		if err != nil {
			return fmt.Errorf("unsupported type %T", x)
		}
		var g any
		if err := json.Unmarshal(blob, &g); err != nil {
			return err
		}
		return encodeAny(w, g, depth+1)
	}
}

func DecodeBinary(b []byte) (any, error) { return decodeAny(bytes.NewBuffer(b), 0) }

func decodeAny(br io.ByteReader, depth int) (any, error) {
	if depth > DefaultLimits.MaxDepth {
		return nil, ErrTooDeep
	}
	tag, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	switch tag {
	case tagNull:
		return nil, nil
	case tagF:
		return false, nil
	case tagT:
		return true, nil
	case tagStr, tagTS, tagDate, tagTime, tagLink, tagAnnot:
		n, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, n)
		for i := 0; i < int(n); i++ {
			bt, e := br.ReadByte()
			if e != nil {
				return nil, e
			}
			buf[i] = bt
		}
		s := string(buf)
		switch tag {
		case tagStr:
			return s, nil
		case tagTS:
			return Timestamp{RFC3339: s}, nil
		case tagDate:
			return Date{YYYYMMDD: s}, nil
		case tagTime:
			return Time{HHMMSS: s}, nil
		case tagLink:
			return Link{Ref: s}, nil
		case tagAnnot:
			return Annot{Note: s}, nil
		}
	case tagBin:
		n, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, n)
		for i := 0; i < int(n); i++ {
			bt, e := br.ReadByte()
			if e != nil {
				return nil, e
			}
			buf[i] = bt
		}
		return Binary(buf), nil
	case tagInt:
		n, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return Int{V: big.NewInt(0)}, nil
		}
		sign, _ := br.ReadByte()
		mag := make([]byte, n-1)
		for i := 0; i < int(n-1); i++ {
			bt, e := br.ReadByte()
			if e != nil {
				return nil, e
			}
			mag[i] = bt
		}
		z := new(big.Int).SetBytes(mag)
		if sign == 0x01 {
			z.Neg(z)
		}
		return Int{V: z}, nil
	case tagDec:
		sign, _ := br.ReadByte()
		exp, err := readZigZag(br)
		if err != nil {
			return nil, err
		}
		ln, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		coef := make([]byte, ln)
		for i := 0; i < int(ln); i++ {
			bt, e := br.ReadByte()
			if e != nil {
				return nil, e
			}
			coef[i] = bt
		}
		var d Decimal
		d.D.Coeff.SetBytes(coef)
		d.D.Exponent = int32(exp)
		d.D.Negative = (sign == 0x01)
		return d, nil
	case tagArr:
		count, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, count)
		for i := 0; i < int(count); i++ {
			v, e := decodeAny(br, depth+1)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
		return out, nil
	case tagObj:
		count, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		obj := make(map[string]any, count)
		for i := 0; i < int(count); i++ {
			ln, e := readUvarint(br)
			if e != nil {
				return nil, e
			}
			kb := make([]byte, ln)
			for j := 0; j < int(ln); j++ {
				bt, ee := br.ReadByte()
				if ee != nil {
					return nil, ee
				}
				kb[j] = bt
			}
			k := string(kb)
			val, ee := decodeAny(br, depth+1)
			if ee != nil {
				return nil, ee
			}
			if k == "$comment" && !PreserveComments {
				continue
			} // keep when true
			obj[k] = val
		}
		return obj, nil
	case tagUUID:
		var u UUID
		for i := 0; i < 16; i++ {
			bt, e := br.ReadByte()
			if e != nil {
				return nil, e
			}
			u[i] = bt
		}
		return u, nil
	case tagSet:
		count, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		out := make(Set, 0, count)
		for i := 0; i < int(count); i++ {
			v, e := decodeAny(br, depth+1)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
		return out, nil
	case tagMap:
		count, err := readUvarint(br)
		if err != nil {
			return nil, err
		}
		out := make(Map, count)
		for i := 0; i < int(count); i++ {
			k, e := decodeAny(br, depth+1)
			if e != nil {
				return nil, e
			}
			v, e := decodeAny(br, depth+1)
			if e != nil {
				return nil, e
			}
			out[k] = v
		}
		return out, nil
	case tagEnv:
		metaAny, err := decodeAny(br, depth+1)
		if err != nil {
			return nil, err
		}
		m, ok := metaAny.(map[string]any)
		if !ok {
			return nil, ErrBadEnvelope
		}
		env := Envelope{}
		if v, ok := m["type"].(string); ok {
			env.Meta.Type = v
		}
		if v, ok := m["schema"].(string); ok {
			env.Meta.Schema = v
		}
		if v, ok := m["version"].(string); ok {
			env.Meta.Version = v
		}
		if v, ok := m["features"].([]any); ok {
			ff := make([]string, 0, len(v))
			for _, it := range v {
				if s, ok := it.(string); ok {
					ff = append(ff, s)
				}
			}
			env.Meta.Features = ff
		}
		if ts, ok := m["createdAt"].(Timestamp); ok {
			env.Meta.Created = &Timestamp{RFC3339: ts.RFC3339}
		} else if v, ok := m["createdAt"].(map[string]any); ok {
			if s, ok := v["value"].(string); ok {
				env.Meta.Created = &Timestamp{RFC3339: s}
			}
		}
		body, err := decodeAny(br, depth+1)
		if err != nil {
			return nil, err
		}
		env.Body = body
		return env, nil
	default:
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnknownTag, tag)
	}
	return nil, fmt.Errorf("jolt: decodeAny fell through (corrupt input?)")
}
