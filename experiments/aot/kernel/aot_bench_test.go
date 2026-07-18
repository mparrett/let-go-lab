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
	var sink int
	for i := 0; i < b.N; i++ {
		sink = MandelBench(nil, 96)
	}
	_ = sink
}
