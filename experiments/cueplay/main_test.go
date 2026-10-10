package main

import (
	"encoding/binary"
	"testing"
)

// The mixer must play the intro once and then repeat the loop with no
// missing or doubled samples at either seam.
func TestIntroThenGaplessLoop(t *testing.T) {
	ramp := func(n, base int) []int16 {
		s := make([]int16, 2*n)
		for i := range s {
			s[i] = int16(base + i)
		}
		return s
	}
	c := &cue{name: "t", intro: ramp(37, 1000), loop: ramp(53, 5000)}
	m := &mixer{}
	m.play(c, 0)
	want := append(append(append([]int16{}, c.intro...), c.loop...), c.loop...)
	buf := make([]byte, len(want)*2)
	// odd-sized reads, like an audio driver's
	for off := 0; off < len(buf); {
		end := min(len(buf), off+4*7)
		n, _ := m.Read(buf[off:end])
		off += n
	}
	for i, w := range want {
		if got := int16(binary.LittleEndian.Uint16(buf[2*i:])); got != w {
			t.Fatalf("sample %d: got %d, want %d", i, got, w)
		}
	}
}
