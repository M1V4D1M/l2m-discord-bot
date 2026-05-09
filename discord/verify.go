package discord

import (
	"crypto/ed25519"
	"encoding/hex"
)

func VerifySignature(publicKeyStr, signatureHex, timestamp, body string) bool {
	publicKey, err := hex.DecodeString(publicKeyStr)
	if err != nil {
		return false
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return false
	}

	msg := []byte(timestamp + body)
	return ed25519.Verify(publicKey, msg, signature)
}
