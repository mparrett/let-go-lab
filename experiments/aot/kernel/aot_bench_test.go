// Benchmark for the AOT-LOWERED kernel. It is NOT compiled by the lab — it's
// dropped into the Go package that `lg-compile` emits from mandel_kernel.lg
// (package mandel_kernel), where it can call the lowered `MandelBench`.
// regen-aot.sh wires this up; see ../README.md. MandelBench is fully native (no
// ec.Invoke), so a nil ExecContext is fine.
package mandel_kernel

import "testing"

func TestMandelAOTValue(t *testing.T) {
	if got := MandelBench(nil, 96); got != 450057 {
		t.Fatalf("AOT kernel = %d, want 450057 (the interpreted result)", got)
	}
}

func BenchmarkMandelAOT(b *testing.B) {
	// int64, with an explicit conversion: the lowered return type widened from
	// int to int64 in let-go v1.13.0 (the 64-bit-across-the-rt-boundary work,
	// nooga/let-go#906). The conversion is a no-op on a build that already
	// returns int64 and widens on an older one, so this template still compiles
	// against both.
	var sink int64
	for i := 0; i < b.N; i++ {
		sink = int64(MandelBench(nil, 96))
	}
	_ = sink
}
