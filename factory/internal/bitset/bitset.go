// Package bitset provides a minimal fixed-size bit set used for story
// coverage bitmaps (00 §9: "known set -> 11k-bit bitmap"). Bit index == word ID.
package bitset

import "math/bits"

// BitSet is a fixed-size bit vector indexed by word ID.
type BitSet struct {
	bits []byte
}

// New returns a BitSet able to hold indices [0, nbits).
func New(nbits int) *BitSet {
	if nbits < 0 {
		nbits = 0
	}
	return &BitSet{bits: make([]byte, (nbits+7)/8)}
}

// FromBytes wraps an existing byte slice (e.g. read from a pack).
func FromBytes(b []byte) *BitSet {
	cp := make([]byte, len(b))
	copy(cp, b)
	return &BitSet{bits: cp}
}

// Set turns on bit i. Out-of-range indices are ignored.
func (b *BitSet) Set(i int) {
	if i < 0 {
		return
	}
	idx := i / 8
	if idx >= len(b.bits) {
		return
	}
	b.bits[idx] |= 1 << uint(i%8)
}

// Test reports whether bit i is on.
func (b *BitSet) Test(i int) bool {
	if i < 0 {
		return false
	}
	idx := i / 8
	if idx >= len(b.bits) {
		return false
	}
	return b.bits[idx]&(1<<uint(i%8)) != 0
}

// Popcount returns the number of set bits.
func (b *BitSet) Popcount() int {
	n := 0
	for _, by := range b.bits {
		n += bits.OnesCount8(by)
	}
	return n
}

// Bytes returns a copy of the underlying storage.
func (b *BitSet) Bytes() []byte {
	cp := make([]byte, len(b.bits))
	copy(cp, b.bits)
	return cp
}

// Len returns the storage size in bytes.
func (b *BitSet) Len() int { return len(b.bits) }
