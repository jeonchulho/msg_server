package service

import (
	"crypto/sha256"
	"encoding/binary"
)

func embed(text string, dim int) []float32 {
	hash := sha256.Sum256([]byte(text))
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		offset := (i * 4) % len(hash)
		chunk := binary.BigEndian.Uint32(hash[offset : offset+4])
		vec[i] = float32(chunk%1000) / 1000.0
	}
	return vec
}
