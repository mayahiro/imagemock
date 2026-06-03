package main

import (
	"math/rand/v2"
	"sync"
)

const seedSalt = 0x9e3779b97f4a7c15

type randomSource struct {
	mu sync.Mutex
	r  *rand.Rand
}

func newRandomSource(seed uint64, seeded bool) *randomSource {
	if !seeded {
		return &randomSource{}
	}
	return &randomSource{
		r: rand.New(rand.NewPCG(seed, seed^seedSalt)),
	}
}

func (r *randomSource) intN(n int) int {
	if r == nil || r.r == nil {
		return rand.IntN(n)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.r.IntN(n)
}
