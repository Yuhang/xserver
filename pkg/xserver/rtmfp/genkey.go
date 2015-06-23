package rtmfp

import (
	"crypto/hmac"
	"crypto/sha256"
)

func ComputeSharedKeys(engine DHEngine, pubkey []byte, initiator []byte) (responder []byte, encrypt, decrypt []byte) {
	var sharedkey []byte
	sharedkey = engine.ComputeSecretKey(pubkey)
	responder = append([]byte{0x03, 0x1a, 0x00, 0x00, 0x02, 0x1e, 0x00, 0x81, 0x02, 0x0d, 0x02}, engine.GetPublicKey()...)
	hashx := hmac.New(sha256.New, sharedkey)
	hash1 := hmac.New(sha256.New, initiator)
	hash1.Write(responder)
	mdp1 := hash1.Sum(nil)
	hash2 := hmac.New(sha256.New, responder)
	hash2.Write(initiator)
	mdp2 := hash2.Sum(nil)
	hashx.Write(mdp1)
	encrypt = hashx.Sum(nil)
	hashx.Reset()
	hashx.Write(mdp2)
	decrypt = hashx.Sum(nil)
	encrypt = encrypt[:AESBlockSize]
	decrypt = decrypt[:AESBlockSize]
	return
}
