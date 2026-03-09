package utils

import (
	"crypto/rand"
	"encoding/base64"
)

func GenRandomToken() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}

// Xorshift implements a simple 32-bit xorshift pseudo-random generator.
type Xorshift struct {
	x, y, z, w uint32
}

// Init initializes the generator with the given seed.
func (xs *Xorshift) Init(seed uint32) {
	xs.x, xs.y, xs.z = 0x6c078965, 0x9908b0df, 0x9d2c5680
	xs.w = seed
}

// Next returns the next pseudo-random value.
func (xs *Xorshift) Next() uint32 {
	t := xs.x ^ (xs.x << 11)
	xs.x, xs.y, xs.z = xs.y, xs.z, xs.w
	xs.w ^= (xs.w >> 19) ^ t ^ (t >> 8)
	return xs.w
}
