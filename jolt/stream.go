package jolt

import (
    "bufio"
    "encoding/binary"
    "io"
)

func WriteFrame(w io.Writer, payload []byte) error {
    var hdr [10]byte
    n := binary.PutUvarint(hdr[:], uint64(len(payload)))
    if _, err := w.Write(hdr[:n]); err != nil { return err }
    _, err := w.Write(payload)
    return err
}

func ReadFrame(r *bufio.Reader) ([]byte, error) {
    n, err := binary.ReadUvarint(r)
    if err != nil { return nil, err }
    buf := make([]byte, n)
    if _, err := io.ReadFull(r, buf); err != nil { return nil, err }
    return buf, nil
}
