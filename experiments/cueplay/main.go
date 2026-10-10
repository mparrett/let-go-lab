// cueplay: play a game music cue through oto — an intro once, then a loop
// forever, with no gap at the seam — and switch cues while it plays. The
// shape a game like TESSERAE needs from prerendered music (the jukebox's
// render.py --loops writes NAME-intro.wav and NAME-loop.wav pairs).
//
//	go run . DIR CUE [CUE...]         # e.g. DIR/02-courses-intro.wav + -loop.wav
//
// Each CUE plays for -each seconds, then crossfades into the next; the last
// one plays until ctrl-c. Builds with CGO_ENABLED=0 on macOS, Linux and
// Windows (oto >= 3.5).
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const rate, chans = 44100, 2

// readPCM returns the samples of a 16-bit stereo 44.1 kHz WAV.
func readPCM(path string) ([]int16, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// find the data chunk; render.py writes a plain 44-byte header, but don't rely on it
	for i := 12; i+8 <= len(b); {
		id, n := string(b[i:i+4]), int(binary.LittleEndian.Uint32(b[i+4:i+8]))
		if id == "fmt " {
			if ch, sr, bits := binary.LittleEndian.Uint16(b[i+10:]), binary.LittleEndian.Uint32(b[i+12:]), binary.LittleEndian.Uint16(b[i+22:]); ch != chans || sr != rate || bits != 16 {
				return nil, fmt.Errorf("%s: want 16-bit stereo 44.1 kHz, got %d-bit %d ch %d Hz", path, bits, ch, sr)
			}
		}
		if id == "data" {
			d := b[i+8 : min(len(b), i+8+n)]
			s := make([]int16, len(d)/2)
			for j := range s {
				s[j] = int16(binary.LittleEndian.Uint16(d[2*j:]))
			}
			return s, nil
		}
		i += 8 + n + n%2
	}
	return nil, fmt.Errorf("%s: no data chunk", path)
}

type cue struct {
	name        string
	intro, loop []int16
}

func loadCue(dir, name string) (*cue, error) {
	c := &cue{name: name}
	var err error
	if c.loop, err = readPCM(filepath.Join(dir, name+"-loop.wav")); err != nil {
		return nil, err
	}
	// a cue may have no intro
	if p := filepath.Join(dir, name+"-intro.wav"); fileExists(p) {
		if c.intro, err = readPCM(p); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// voice reads one cue: the intro once, then the loop with its read position
// wrapping, so the seam is sample-exact (no decoder or file boundary in between).
type voice struct {
	c       *cue
	pos     int
	inLoop  bool
	gain    float64 // current level, 0..1
	target  float64
	perSamp float64 // gain change per sample while fading
}

func (v *voice) next() (l, r float64) {
	src := v.c.intro
	if v.inLoop || len(src) == 0 {
		src, v.inLoop = v.c.loop, true
	}
	l, r = float64(src[v.pos]), float64(src[v.pos+1])
	v.pos += 2
	if v.pos >= len(src) {
		v.pos, v.inLoop = 0, true
	}
	if v.gain != v.target {
		if v.gain < v.target {
			v.gain = min(v.target, v.gain+v.perSamp)
		} else {
			v.gain = max(v.target, v.gain-v.perSamp)
		}
	}
	return l * v.gain, r * v.gain
}

// mixer is the one io.Reader oto pulls from; cue changes swap voices under a lock.
type mixer struct {
	mu     sync.Mutex
	voices []*voice
}

func (m *mixer) play(c *cue, fade time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	step := 1.0
	if fade > 0 {
		step = 1 / (fade.Seconds() * rate)
	}
	for _, v := range m.voices {
		v.target, v.perSamp = 0, step
	}
	v := &voice{c: c, target: 1, perSamp: step, gain: 1}
	if fade > 0 {
		v.gain = 0
	}
	m.voices = append(m.voices, v)
}

func (m *mixer) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(p) / 4 * 4
	for i := 0; i < n; i += 4 {
		var l, r float64
		for _, v := range m.voices {
			vl, vr := v.next()
			l, r = l+vl, r+vr
		}
		binary.LittleEndian.PutUint16(p[i:], uint16(int16(clamp(l))))
		binary.LittleEndian.PutUint16(p[i+2:], uint16(int16(clamp(r))))
	}
	// drop voices that have faded out
	live := m.voices[:0]
	for _, v := range m.voices {
		if v.gain > 0 || v.target > 0 {
			live = append(live, v)
		}
	}
	m.voices = live
	return n, nil
}

func clamp(x float64) float64 { return max(-32768, min(32767, x)) }

// out holds the player for the life of the program: oto closes a Player once its
// handle is unreachable, even mid-playback (documented since oto 3.5.1,
// ebitengine/oto#293). A field, so playerlifetime_test.go can see it's kept.
var out struct{ player *oto.Player }

func main() {
	each := flag.Duration("each", 20*time.Second, "how long each cue plays before the next")
	fade := flag.Duration("fade", 2*time.Second, "crossfade between cues")
	render := flag.String("render", "", "write what would play to this WAV instead of the speakers")
	length := flag.Duration("length", 90*time.Second, "with -render: how much to write")
	flag.Parse()
	if flag.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: cueplay [-each 20s] [-fade 2s] DIR CUE [CUE...]")
		os.Exit(2)
	}
	dir := flag.Arg(0)
	var cues []*cue
	for _, name := range flag.Args()[1:] {
		c, err := loadCue(dir, name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cues = append(cues, c)
	}

	if *render != "" {
		if err := renderWAV(*render, cues, *each, *fade, *length); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: rate, ChannelCount: chans, Format: oto.FormatSignedInt16LE})
	if err != nil {
		fmt.Fprintln(os.Stderr, "audio:", err)
		os.Exit(1)
	}
	<-ready
	m := &mixer{}
	m.play(cues[0], 0)
	out.player = ctx.NewPlayer(m)
	out.player.Play()
	for i, c := range cues {
		if i > 0 {
			m.play(c, *fade)
		}
		fmt.Printf("playing %s (intro %.1f s, loop %.1f s)\n", c.name, float64(len(c.intro))/2/rate, float64(len(c.loop))/2/rate)
		if i < len(cues)-1 {
			time.Sleep(*each)
		}
	}
	select {} // the last cue loops until ctrl-c
}

// renderWAV runs the same mixer and cue schedule as live playback, pulling
// samples the way oto would, and writes them to a 16-bit stereo WAV.
func renderWAV(path string, cues []*cue, each, fade, length time.Duration) error {
	m := &mixer{}
	m.play(cues[0], 0)
	total := int(length.Seconds()*rate) * 4
	pcm := make([]byte, 0, total)
	buf := make([]byte, 4096)
	for next := 1; len(pcm) < total; {
		// switch cues at the same moments live playback would
		if next < len(cues) && len(pcm) >= int(float64(next)*each.Seconds()*rate)*4 {
			m.play(cues[next], fade)
			next++
		}
		n, _ := m.Read(buf[:min(len(buf), total-len(pcm))])
		pcm = append(pcm, buf[:n]...)
	}
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], chans)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], rate*chans*2)
	binary.LittleEndian.PutUint16(h[32:], chans*2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	return os.WriteFile(path, append(h, pcm...), 0o644)
}

var _ io.Reader = (*mixer)(nil)
