// Copyright The RAI Inc.
// The RAI Authors
package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// This function will encrypt a text using AES in base64. The `key` a.k.a passphrase is also encoded using base64.
func BeanAESEncrypt(key, plainText string) (string, error) {

	keyInByte, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	plainTextByte := []byte(plainText)
	plainTextByte, err = Pkcs7Pad(plainTextByte, aes.BlockSize)
	if err != nil {
		return "", err
	}

	if len(plainTextByte)%aes.BlockSize != 0 {
		err := fmt.Errorf(`plainText: "%s" has the wrong block size`, plainTextByte)
		return "", err
	}

	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		return "", err
	}

	cipherBlock, err := aes.NewCipher(keyInByte)
	if err != nil {
		return "", err
	}

	encryptedTextByte := make([]byte, len(plainTextByte))

	mode := cipher.NewCBCEncrypter(cipherBlock, iv)
	mode.CryptBlocks(encryptedTextByte, plainTextByte)

	encryptedTextBase64 := base64.StdEncoding.EncodeToString(encryptedTextByte)
	ivTextBase64 := base64.StdEncoding.EncodeToString(iv)

	data := ivTextBase64 + encryptedTextBase64
	mac := ComputeHmacSha256(data, string(keyInByte))

	ticket := make(map[string]interface{})
	ticket["iv"] = ivTextBase64
	ticket["mac"] = mac
	ticket["value"] = encryptedTextBase64

	resTicket, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}

	ticketR := base64.StdEncoding.EncodeToString(resTicket)

	return ticketR, nil
}

// This function will decrypt an AES encrypted text in base64. The `key` a.k.a passphrase is also encoded using base64.
func BeanAESDecrypt(key, encryptedText string) (string, error) {

	base64DecodedByteText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	var p map[string]interface{}

	if err := json.Unmarshal(base64DecodedByteText, &p); err != nil {
		return "", err
	}

	iv, err := base64.StdEncoding.DecodeString(p["iv"].(string))
	if err != nil {
		return "", err
	}

	encryptedValue, err := base64.StdEncoding.DecodeString(p["value"].(string))
	if err != nil {
		return "", err
	}

	keyInByte, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	cipherBlock, err := aes.NewCipher(keyInByte)
	if err != nil {
		return "", err
	}

	cipher.NewCBCDecrypter(cipherBlock, iv).CryptBlocks(encryptedValue, encryptedValue)

	out, err := Pkcs7Unpad(encryptedValue, aes.BlockSize)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// Pkcs7Pad right-pads the given byte slice with 1 to n bytes, where
// n is the block size. The size of the result is x times n, where x
// is at least 1.
func Pkcs7Pad(data []byte, blocklen int) ([]byte, error) {

	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("invalid data len %d", len(data))
	}

	n := blocklen - (len(data) % blocklen)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))

	return pb, nil
}

// Pkcs7Unpad validates and unpads data from the given bytes slice.
// The returned value will be 1 to n bytes smaller depending on the
// amount of padding, where n is the block size.
func Pkcs7Unpad(data []byte, blocklen int) ([]byte, error) {

	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}

	if len(data)%blocklen != 0 || len(data) == 0 {
		return nil, fmt.Errorf("invalid data len %d", len(data))
	}

	padlen := int(data[len(data)-1])
	if padlen > blocklen || padlen == 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	pad := data[len(data)-padlen:]
	for i := 0; i < padlen; i++ {
		if pad[i] != byte(padlen) {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return data[:len(data)-padlen], nil
}

func ComputeHmacSha256(message string, secret string) string {

	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	sha := hex.EncodeToString(h.Sum(nil))

	return sha
}
