package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// wav builds a WAV file from raw chunks, so tests can write broken ones.
func wav(chunks ...[]byte) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WAVE")
	for _, c := range chunks {
		b = append(b, c...)
	}
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	return b
}

func chunk(id string, declared int, body []byte) []byte {
	b := append([]byte(id), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(b[4:], uint32(declared))
	return append(b, body...)
}

func fmtBody(tag, ch uint16, sr uint32, bits uint16) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:], tag)
	binary.LittleEndian.PutUint16(b[2:], ch)
	binary.LittleEndian.PutUint32(b[4:], sr)
	binary.LittleEndian.PutUint32(b[8:], sr*uint32(ch)*2)
	binary.LittleEndian.PutUint16(b[12:], ch*2)
	binary.LittleEndian.PutUint16(b[14:], bits)
	return b
}

// A malformed file must be a load error, never a panic in the mixer.
func TestReadPCMRejectsMalformedFiles(t *testing.T) {
	good := fmtBody(1, chans, rate, 16)
	cases := map[string]struct {
		file []byte
		want string
	}{
		"not riff":        {[]byte("hello, this is not a wav"), "not a WAV"},
		"short":           {[]byte("RIFF"), "not a WAV"},
		"fmt cut off":     {wav(chunk("fmt ", 16, good[:6])), "fmt chunk is 6 bytes"},
		"data before fmt": {wav(chunk("data", 8, make([]byte, 8)), chunk("fmt ", 16, good)), "before fmt"},
		"no fmt":          {wav(chunk("data", 8, make([]byte, 8))), "before fmt"},
		"mono":            {wav(chunk("fmt ", 16, fmtBody(1, 1, rate, 16)), chunk("data", 8, make([]byte, 8))), "want 16-bit"},
		"float":           {wav(chunk("fmt ", 16, fmtBody(3, chans, rate, 16)), chunk("data", 8, make([]byte, 8))), "want 16-bit"},
		"empty data":      {wav(chunk("fmt ", 16, good), chunk("data", 0, nil)), "no audio"},
		"less than frame": {wav(chunk("fmt ", 16, good), chunk("data", 400, make([]byte, 3))), "no audio"},
		"no data chunk":   {wav(chunk("fmt ", 16, good)), "no data chunk"},
	}
	dir := t.TempDir()
	for name, tc := range cases {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".wav")
		if err := os.WriteFile(p, tc.file, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := readPCM(p)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", name, err, tc.want)
		}
	}
}

// A truncated data chunk keeps its whole frames and drops the partial one.
func TestReadPCMTruncatedDataKeepsWholeFrames(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cut.wav")
	body := []byte{1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6} // two frames and a stray 3 bytes
	if err := os.WriteFile(p, wav(chunk("fmt ", 16, fmtBody(1, chans, rate, 16)), chunk("data", 400, body)), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := readPCM(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int16{1, 2, 3, 4}; len(s) != len(want) || s[0] != 1 || s[3] != 4 {
		t.Fatalf("got %v, want %v", s, want)
	}
}

func readSamples(t *testing.T, m *mixer, n int) []int16 {
	t.Helper()
	buf := make([]byte, 4*n)
	if got, err := m.Read(buf); err != nil || got != len(buf) {
		t.Fatalf("Read: %d, %v", got, err)
	}
	s := make([]int16, 2*n)
	for i := range s {
		s[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
	}
	return s
}

func constant(frames int, v int16) []int16 {
	s := make([]int16, 2*frames)
	for i := range s {
		s[i] = v
	}
	return s
}

// A crossfade ramps the old cue down and the new one up over the fade, then
// drops the old voice.
func TestCrossfade(t *testing.T) {
	a := &cue{name: "a", loop: constant(100, 1000)}
	b := &cue{name: "b", loop: constant(100, 3000)}
	m := &mixer{}
	m.play(a, 0)
	readSamples(t, m, 10)
	fadeFrames := 50
	m.play(b, time.Duration(float64(fadeFrames)/rate*float64(time.Second)))
	s := readSamples(t, m, fadeFrames/2)
	mid := s[len(s)-2]
	if mid <= 1000 || mid >= 3000 {
		t.Fatalf("halfway through the fade got %d, want between the two cues", mid)
	}
	s = readSamples(t, m, fadeFrames)
	if last := s[len(s)-2]; last != 3000 {
		t.Fatalf("after the fade got %d, want 3000", last)
	}
	if len(m.voices) != 1 || m.voices[0].c != b {
		t.Fatalf("after the fade %d voices remain, want only b", len(m.voices))
	}
}

// A cue without an intro starts straight in its loop.
func TestNoIntro(t *testing.T) {
	c := &cue{name: "n", loop: []int16{1, 2, 3, 4, 5, 6}}
	m := &mixer{}
	m.play(c, 0)
	got := readSamples(t, m, 5)
	want := []int16{1, 2, 3, 4, 5, 6, 1, 2, 3, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestReadShorterThanAFrame(t *testing.T) {
	m := &mixer{}
	m.play(&cue{name: "c", loop: []int16{1, 2}}, 0)
	if n, err := m.Read(make([]byte, 3)); n != 0 || !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("got %d, %v; want 0, io.ErrShortBuffer", n, err)
	}
}

// renderWAV writes a readable WAV of the requested length.
func TestRenderWAV(t *testing.T) {
	a := &cue{name: "a", intro: constant(10, 500), loop: constant(20, 1000)}
	b := &cue{name: "b", loop: constant(20, 2000)}
	p := filepath.Join(t.TempDir(), "out.wav")
	if err := renderWAV(p, []*cue{a, b}, 50*time.Millisecond, 10*time.Millisecond, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	s, err := readPCM(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := int(0.2*rate) * chans; len(s) != want {
		t.Fatalf("got %d samples, want %d", len(s), want)
	}
	if s[0] != 500 || s[len(s)-1] != 2000 {
		t.Fatalf("starts at %d and ends at %d, want 500 (a's intro) and 2000 (b)", s[0], s[len(s)-1])
	}
}
