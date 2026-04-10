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

	// 3. Получаем "Raw" приватный ключ (просто набор байтов)
	// D — это и есть числовое значение приватного ключа
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
	// Собираем координаты X и Y в байты
	data := append(pub.X.Bytes(), pub.Y.Bytes()...)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16]) // Берем первые 16 байт для краткости
}
