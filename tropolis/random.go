package tropolis

import "math/rand/v2"

const (
	letters    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lettersLen = len(letters)
)

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(lettersLen)]
	}
	return string(b)
}
