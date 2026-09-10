package lifecycle

import (
    "crypto/sha256"
    "encoding/hex"
)

func VerifyPayloadSHA256(payload []byte, expected string) bool {
    sum := sha256.Sum256(payload)
    return hex.EncodeToString(sum[:]) == expected
}
