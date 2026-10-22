package pramaan

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strings"
)

// requestKey keeps transport identifiers independent of application libraries.
func requestKey(value string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(value, "urn:uuid:"), "}"), "{"), "-", "")
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 16 {
		return value, nil
	}
	var key [16]byte
	if value == "" {
		if _, err := rand.Read(key[:]); err != nil {
			return "", errors.New("A request reference could not be created.")
		}
		key[6] = (key[6] & 0x0f) | 0x40
	} else {
		// URL namespace from RFC 4122; preserve existing deterministic request keys.
		namespace := []byte{0x6b, 0xa7, 0xb8, 0x11, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
		sum := sha1.Sum(append(namespace, []byte(value)...))
		copy(key[:], sum[:16])
		key[6] = (key[6] & 0x0f) | 0x50
	}
	key[8] = (key[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(key[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}
