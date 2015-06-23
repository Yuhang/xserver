package rtmfp

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const (
	AESBlockSize = aes.BlockSize
)

type AESEngine interface {
	SetKey(encrypt, decrypt []byte) error
	Encode(data []byte) error
	Decode(data []byte) error
}

type aesEngine struct {
	ecb, dcb cipher.Block
}

func NewAESEngine() *aesEngine {
	return &aesEngine{}
}

func (e *aesEngine) SetKey(encrypt, decrypt []byte) (err error) {
	var ecb, dcb cipher.Block
	if len(encrypt) != AESBlockSize {
		return errors.New("encrypt.bad key size")
	} else if ecb, err = aes.NewCipher(encrypt); err != nil {
		return errors.New("encrypt.create error")
	}
	if len(decrypt) != AESBlockSize {
		return errors.New("decrypt.bad key size")
	} else if dcb, err = aes.NewCipher(decrypt); err != nil {
		return errors.New("decrypt.create error")
	}
	e.ecb, e.dcb = ecb, dcb
	return
}

func (e *aesEngine) Encode(data []byte) (err error) {
	if len(data) == 0 {
		return
	} else if len(data)%AESBlockSize != 0 {
		return errors.New("encode.bad data length")
	}
	mode := cipher.NewCBCEncrypter(e.ecb, make([]byte, AESBlockSize))
	mode.CryptBlocks(data, data)
	return
}

func (e *aesEngine) Decode(data []byte) (err error) {
	if len(data) == 0 {
		return
	} else if len(data)%AESBlockSize != 0 {
		return errors.New("decode.bad data length")
	}
	mode := cipher.NewCBCDecrypter(e.dcb, make([]byte, AESBlockSize))
	mode.CryptBlocks(data, data)
	return
}
