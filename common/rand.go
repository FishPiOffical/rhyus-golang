package common

import (
	"math/rand"
	"time"
)

// String returns a random string ['a', 'z'] and ['0', '9'] in the specified length.
func (g *GuRand) String(length int) string {
	src := rand.NewSource(time.Now().UnixMilli())
	randGenerator := rand.New(src)
	letter := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, length)
	for i := range b {
		b[i] = letter[randGenerator.Intn(len(letter))]
	}
	return string(b)
}

// Int returns a random integer in range [min, max].
func (g *GuRand) Int(min int, max int) int {
	src := rand.NewSource(time.Now().UnixMilli())
	randGenerator := rand.New(src)
	return min + randGenerator.Intn(max-min)
}
