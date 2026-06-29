package native

import "testing"

// Sanity bound, not exact: the demo's coordinate setup uses Clojure-style
// single-precision (float ...) while this port is float64, so a handful of
// escape-boundary pixels differ (~0.09%). Same work; assert within 1% of the
// demo's sum=450584 / bytes=27695.
func TestFrameMatchesDemo(t *testing.T) {
	grid, sum := computeGrid(homeX, homeY, homeW, 96)
	s := encodeFrame(grid)
	if abs(sum-450584) > 4506 {
		t.Fatalf("iter sum = %d, want ~450584 (within 1%%)", sum)
	}
	if abs(len(s)-27695) > 277 {
		t.Fatalf("frame bytes = %d, want ~27695 (within 1%%)", len(s))
	}
	t.Logf("native frame: bytes=%d sum=%d (demo: 27695 / 450584)", len(s), sum)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func BenchmarkComputeNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		computeGrid(homeX, homeY, homeW, 96)
	}
}

func BenchmarkEncodeNative(b *testing.B) {
	grid, _ := computeGrid(homeX, homeY, homeW, 96)
	b.ResetTimer()
	var sink int
	for i := 0; i < b.N; i++ {
		sink += len(encodeFrame(grid))
	}
	_ = sink
}

func BenchmarkFrameNative(b *testing.B) {
	for i := 0; i < b.N; i++ {
		grid, _ := computeGrid(homeX, homeY, homeW, 96)
		encodeFrame(grid)
	}
}
