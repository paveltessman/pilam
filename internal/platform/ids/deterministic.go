package ids

import (
	"encoding/binary"
	"math/rand/v2"
	"sync"
)

// The deterministic sequence counts from: 2020-01-01T00:00:00Z, in Unix milliseconds.
// Any fixed instant would do.
const deterministicEpochMillis uint64 = 1_577_836_800_000

// The PCG state. A generator seeded with 0 would otherwise
// start from an all-zero state, which PCG needs a few draws to escape.
const deterministicStream uint64 = 0x9E37_79B9_7F4A_7C15

// NewDeterministic returns a determenistic generator that yields the same identifiers,
// in the same order, on every run, every machine and every build.
//
// Nothing outside the seed and the tests should use this.
func NewDeterministic(seed uint64) Generator {
	return &deterministicGenerator{rng: rand.New(rand.NewPCG(seed, deterministicStream))}
}

type deterministicGenerator struct {
	mu      sync.Mutex
	rng     *rand.Rand
	counter uint64
}

func (g *deterministicGenerator) New() ID {
	g.mu.Lock()
	defer g.mu.Unlock()

	// One millisecond per identifier. Uniqueness and ordering then come from
	// the counter rather than from the random bits.
	millis := deterministicEpochMillis + g.counter
	g.counter++

	var b [16]byte
	// Bytes 0-5 are the 48-bit timestamp; the low 16 bits of the same word are
	// rand_a, of which the top four are overwritten by the version below.
	binary.BigEndian.PutUint64(b[0:8], millis<<16|uint64(g.rng.Uint32()&0xffff))
	binary.BigEndian.PutUint64(b[8:16], g.rng.Uint64())

	b[6] = b[6]&0x0f | 0x70 // version 7
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant

	return b
}
