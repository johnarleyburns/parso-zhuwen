package bitset

import "testing"

func TestSetTestPopcount(t *testing.T) {
	b := New(20)
	if b.Len() != 3 { // ceil(20/8)=3 bytes
		t.Fatalf("len = %d, want 3", b.Len())
	}
	for _, i := range []int{0, 4, 11, 19} {
		b.Set(i)
	}
	for _, i := range []int{0, 4, 11, 19} {
		if !b.Test(i) {
			t.Errorf("bit %d not set", i)
		}
	}
	if b.Test(1) {
		t.Error("bit 1 unexpectedly set")
	}
	if b.Popcount() != 4 {
		t.Errorf("popcount = %d, want 4", b.Popcount())
	}
}

func TestOutOfRangeIgnored(t *testing.T) {
	b := New(8)
	b.Set(-1)
	b.Set(1000)
	if b.Popcount() != 0 {
		t.Errorf("popcount = %d, want 0", b.Popcount())
	}
	if b.Test(1000) || b.Test(-1) {
		t.Error("out-of-range Test should be false")
	}
}

func TestBytesRoundTrip(t *testing.T) {
	b := New(16)
	b.Set(3)
	b.Set(9)
	cp := FromBytes(b.Bytes())
	if !cp.Test(3) || !cp.Test(9) || cp.Popcount() != 2 {
		t.Error("round trip via Bytes/FromBytes failed")
	}
	// Bytes returns a copy; mutating it must not affect the set.
	raw := b.Bytes()
	raw[0] = 0xFF
	if b.Popcount() != 2 {
		t.Error("Bytes did not return a copy")
	}
}
