package joltsec

import "fmt"

type Keyring interface {
    Get(keyID string) ([]byte, error)
}

type StaticKeyring map[string][]byte

func (s StaticKeyring) Get(keyID string) ([]byte, error) {
    k, ok := s[keyID]
    if !ok { return nil, fmt.Errorf("key %q not found", keyID) }
    return k, nil
}
