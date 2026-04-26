package object

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// Header builds the Git object header: "<type> <size>\0".
func Header(t Type, contentLen int) []byte {
	h := fmt.Sprintf("%s %d\x00", t, contentLen)
	return []byte(h)
}

// HashBytes computes the SHA-1 of the full object data (header + content).
func HashBytes(t Type, content []byte) ([]byte, string) {
	header := Header(t, len(content))
	full := append(header, content...)
	sum := sha1.Sum(full)
	return sum[:], hex.EncodeToString(sum[:])
}

// HashHex computes and returns only the hex string.
func HashHex(t Type, content []byte) string {
	_, hex := HashBytes(t, content)
	return hex
}

// ValidateHash checks a hex hash string is exactly 40 lowercase hex chars.
func ValidateHash(h string) bool {
	if len(h) != 40 {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}
