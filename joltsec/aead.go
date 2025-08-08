package joltsec

import (
    "crypto/aes"
    "crypto/cipher"
    "errors"

    "golang.org/x/crypto/chacha20poly1305"
)

type Alg string

const (
    AlgXChaCha20Poly1305 Alg = "XCHACHA20-POLY1305"
    AlgAES256GCM         Alg = "AES-256-GCM"
)

type aeadSuite struct {
    alg      Alg
    keyLen   int
    nonceLen int
    newAEAD  func(key []byte) (cipher.AEAD, error)
}

var suites = map[Alg]aeadSuite{
    AlgXChaCha20Poly1305: {
        alg:      AlgXChaCha20Poly1305,
        keyLen:   chacha20poly1305.KeySize,
        nonceLen: chacha20poly1305.NonceSizeX,
        newAEAD: func(key []byte) (cipher.AEAD, error) { return chacha20poly1305.NewX(key) },
    },
    AlgAES256GCM: {
        alg:      AlgAES256GCM,
        keyLen:   32,
        nonceLen: 12,
        newAEAD: func(key []byte) (cipher.AEAD, error) {
            block, err := aes.NewCipher(key)
            if err != nil { return nil, err }
            return cipher.NewGCM(block)
        },
    },
}

func suiteFor(alg Alg) (aeadSuite, error) {
    s, ok := suites[alg]
    if !ok { return aeadSuite{}, errors.New("unsupported AEAD algorithm") }
    return s, nil
}
