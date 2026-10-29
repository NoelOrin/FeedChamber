package pkg

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateNumericCode returns a cryptographically random n-digit numeric string,
// zero-padded on the left (e.g. "042891" for n=6).
func GenerateNumericCode(n int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	v, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%0*d", n, v)
}
