package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"slices"
)

func main() {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Get the "raw" private key (just a byte sequence): the scalar at the
	// curve's fixed width.
	rawPrivate, err := privateKey.Bytes()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Raw Private Key (hex): %x\n", rawPrivate)

	rawPublicKey, err := privateKey.PublicKey.Bytes()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Raw Public Key (hex): %x\n", rawPublicKey)
	fmt.Printf("KID: %s\n", generateKID(&privateKey.PublicKey))
}

func generateKID(pub *ecdsa.PublicKey) string {
	point, err := pub.Bytes()
	if err != nil {
		log.Fatal(err)
	}

	// Concatenate the X and Y coordinates, each without leading zero bytes --
	// the form the KID has always been derived from, so it stays stable.
	const coordBytes = 32
	x := bytes.TrimLeft(point[1:1+coordBytes], "\x00")
	y := bytes.TrimLeft(point[1+coordBytes:], "\x00")
	data := append(slices.Clone(x), y...)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16]) // Take the first 16 bytes for brevity
}
