package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
)

func main() {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Get the "raw" private key (just a byte sequence).
	// D is the numeric value of the private key itself.
	rawPrivate := privateKey.D.Bytes()

	fmt.Printf("Raw Private Key (hex): %x\n", rawPrivate)

	rawPublicKey, err := privateKey.PublicKey.Bytes()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Raw Public Key (hex): %x\n", rawPublicKey)
	fmt.Printf("KID: %s\n", generateKID(&privateKey.PublicKey))
}

func generateKID(pub *ecdsa.PublicKey) string {
	// Concatenate the X and Y coordinates into bytes
	data := append(pub.X.Bytes(), pub.Y.Bytes()...)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16]) // Take the first 16 bytes for brevity
}
