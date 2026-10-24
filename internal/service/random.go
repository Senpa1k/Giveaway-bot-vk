package service

import (
	crand "crypto/rand"
	"fmt"
	"math/big"

	"github.com/google/uuid"
)

// shuffleIDs puts the ids in crypto-random order. Raffle order must not be
// reproducible from the clock or the process seed, so math/rand is not used.
func shuffleIDs(ids []uuid.UUID) error {
	for i := len(ids) - 1; i > 0; i-- {
		j, err := cryptoInt(i + 1)
		if err != nil {
			return fmt.Errorf("read draw randomness: %w", err)
		}
		ids[i], ids[j] = ids[j], ids[i]
	}
	return nil
}

// cryptoInt returns a uniform random value in [0, n).
func cryptoInt(n int) (int, error) {
	value, err := crand.Int(crand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("read crypto random: %w", err)
	}
	return int(value.Int64()), nil
}
