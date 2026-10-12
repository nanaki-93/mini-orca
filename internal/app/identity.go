package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func newOpaqueID(prefix string) string {
	var token [16]byte
	if _, err := rand.Read(token[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(token[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}
