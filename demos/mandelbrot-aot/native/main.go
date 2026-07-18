package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	k "mandelbrot-aot/gen/aot/kernel"

	"golang.org/x/term"
)

const ramp = " .,:;irsXA253hMHGS#9B&@"

// buf is reused across frames — the renderer allocates nothing per frame, so
// GC stays quiet and the zoom doesn't hitch. (The hitch was my per-cell
// fmt.Fprintf, not the kernel.)
var buf = make([]byte, 0, 100*44*24)

// colorOn=false renders plain ASCII with an mi-INDEPENDENT char mapping (char
// from the raw iteration count, not it/mi) — a control view free of the
// maxiter-coupled coloring. Toggle via the `ascii` mode or PLAIN=1.
var colorOn = true

func appendCell(b []byte, r, g, bl int, ch byte) []byte {
	b = append(b, "\x1b[38;2;"...)
	b = strconv.AppendInt(b, int64(r), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(g), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(bl), 10)
	b = append(b, 'm', ch)
	return b
}

func render(W, H int, cx0, cy0, spanX float64, mi int) []byte {
	buf = buf[:0]
	spanY := spanX * float64(H) / float64(W) / 0.5
	for row := 0; row < H; row++ {
		cy := cy0 - spanY/2 + spanY*float64(row)/float64(H)
		for col := 0; col < W; col++ {
			cx := cx0 - spanX/2 + spanX*float64(col)/float64(W)
			it := k.Escape(nil, cx, cy, mi)
			if it >= mi { // interior
				if colorOn {
					buf = append(buf, "\x1b[38;2;10;10;20m "...)
				} else {
					buf = append(buf, ' ')
				}
				continue
			}
			if colorOn {
				t := float64(it) / float64(mi)
				buf = appendCell(buf, int(9+246*t), int(20+120*t*t), int(120+135*(1-t)), ramp[int(t*float64(len(ramp)-1))])
			} else {
				// mi-independent: char from raw iteration count → no maxiter coupling.
				buf = append(buf, ramp[it%len(ramp)])
			}
		}
		if colorOn {
			buf = append(buf, "\x1b[0m\n"...)
		} else {
			buf = append(buf, '\n')
		}
	}
	return buf
}

// Interactive control model, ported from demos/mandelbrot/mandelbrot.lg (the VM
// demo). Same home view and same key bindings, so the two feel identical — this
// one just runs the lowered native Escape instead of the bytecode VM.
const (
	homeX, homeY, homeW = -0.6, 0.0, 3.2 // home view: center + span (view width)
	homeMaxiter         = 96
	zoomFactor          = 0.7 // +/- multiply/divide the span; 0.7 ≈ a 1.4× step
	panFrac             = 0.2 // one pan step = 20% of the current span
	miStep              = 16
	miMin, miMax        = 16, 1024
)

// readKey decodes one keypress from a raw-mode stdin: a single byte, Ctrl-C, or
// a 3-byte arrow escape sequence. It only consumes the trailing bytes of an
// escape sequence when they're already buffered, so a lone ESC never blocks
// waiting for more (arrow keys arrive as one ESC-[-A burst, so they are buffered).
func readKey(r *bufio.Reader) (string, error) {
	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch b {
	case 3: // Ctrl-C
		return "q", nil
	case 0x1b: // ESC — an arrow sequence if the rest already arrived, else a no-op
		// Only consume bytes already buffered, so a lone ESC never blocks (arrow
		// keys arrive as one ESC-[-x burst). Drain the whole CSI to its final byte
		// (0x40..0x7e) so an unrecognized sequence — modified arrows, Home/PgUp —
		// leaves no tail to be misread as later keystrokes; only a bare ESC-[-x
		// maps to an arrow.
		if r.Buffered() >= 2 {
			if b2, _ := r.ReadByte(); b2 == '[' {
				var seq []byte
				for r.Buffered() > 0 {
					c, _ := r.ReadByte()
					seq = append(seq, c)
					if c >= 0x40 && c <= 0x7e { // CSI final byte
						break
					}
				}
				if len(seq) == 1 {
					switch seq[0] {
					case 'A':
						return "up", nil
					case 'B':
						return "down", nil
					case 'C':
						return "right", nil
					case 'D':
						return "left", nil
					}
				}
			}
		}
		return "", nil
	default:
		return string(b), nil
	}
}

// runInteractive is the "grow the showcase into the full fractal" mode (#35): a
// live keyboard-driven fractal at native speed. Rendering, sixel-free ANSI cells,
// and DEC-2026 synchronized output are the same machinery the scripted `zoom`
// mode already uses; the new part is the raw-mode read-key loop.
func runInteractive(out *bufio.Writer) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintln(os.Stderr, "interactive mode needs a TTY on stdin (try a real terminal, not a pipe)")
		os.Exit(1)
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "interactive: cannot enter raw mode:", err)
		os.Exit(1)
	}
	// One teardown path for every exit (return, EOF, signal): restore cooked
	// mode, leave the alt screen, show the cursor, reset SGR. sync.Once so the
	// deferred call and the signal goroutine can both invoke it without racing
	// (and it runs exactly once). It writes to os.Stdout, not the buffered `out`,
	// so a signal mid-frame can't corrupt the writer's shared buffer.
	var once sync.Once
	restore := func() {
		once.Do(func() {
			term.Restore(fd, old)
			os.Stdout.WriteString("\x1b[?2026l\x1b[?25h\x1b[0m\x1b[?1049l")
		})
	}
	defer restore()
	// Raw mode disables ISIG, so Ctrl-C arrives as a byte (handled in readKey);
	// this net catches a SIGTERM (or SIGINT that slips through) so the terminal
	// is never left raw with the cursor hidden.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sigCh; restore(); os.Exit(130) }()

	out.WriteString("\x1b[?1049h\x1b[?25l\x1b[2J") // alt screen, hide cursor, clear
	out.Flush()

	const W, H = 100, 44
	cx, cy, vw, mi := homeX, homeY, homeW, homeMaxiter
	r := bufio.NewReader(os.Stdin)
	for {
		t0 := time.Now()
		frame := render(W, H, cx, cy, vw, mi)
		ms := float64(time.Since(t0).Microseconds()) / 1000.0
		// DEC 2026: present image + status atomically, no mid-draw tearing.
		out.WriteString("\x1b[?2026h\x1b[H")
		out.Write(frame)
		out.WriteString("\x1b[0m x=")
		out.WriteString(strconv.FormatFloat(cx, 'f', 5, 64))
		out.WriteString(" y=")
		out.WriteString(strconv.FormatFloat(cy, 'f', 5, 64))
		out.WriteString(" zoom=")
		out.WriteString(strconv.Itoa(int(homeW / vw)))
		out.WriteString("×  maxiter=")
		out.WriteString(strconv.Itoa(mi))
		out.WriteString("  compute=")
		out.WriteString(strconv.FormatFloat(ms, 'f', 1, 64))
		out.WriteString("ms\x1b[K\n [+/-] zoom  [hjkl/arrows] pan  [,/.] detail  [r] reset  [q] quit\x1b[K")
		out.WriteString("\x1b[?2026l")
		out.Flush()

		key, err := readKey(r)
		if err != nil { // EOF on stdin
			return
		}
		pan := vw * panFrac
		switch key {
		case "q":
			return
		case "+", "=":
			vw *= zoomFactor
		case "-", "_":
			vw /= zoomFactor
		case "h", "left":
			cx -= pan
		case "l", "right":
			cx += pan
		case "k", "up":
			cy -= pan
		case "j", "down":
			cy += pan
		case ".":
			mi = min(miMax, mi+miStep)
		case ",":
			mi = max(miMin, mi-miStep)
		case "r":
			cx, cy, vw, mi = homeX, homeY, homeW, homeMaxiter
		}
	}
}

func main() {
	mode := "bench"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if os.Getenv("PLAIN") == "1" {
		colorOn = false // applies to render + zoom
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	switch mode {
	case "interactive", "play", "i":
		runInteractive(out)

	case "render":
		out.Write(render(100, 44, -0.75, 0.0, 3.2, 300))

	case "ascii": // static plain-ASCII render (mi-independent chars, no color)
		colorOn = false
		out.Write(render(100, 44, -0.75, 0.0, 3.2, 300))

	case "zoom":
		// zoom [frames] [startFrame] [delayMs] [stepPct]
		//   frames     how many frames to play        (default 240)
		//   startFrame jump straight to this depth     (default 0)
		//   delayMs    sleep per frame; raise to slow  (default 33 ≈ 30fps)
		//   stepPct    zoom factor/frame as %; raise toward 100 to crawl (default 96.5)
		// Each frame prints its ABSOLUTE frame index + span + mi so you can report
		// exactly where a discontinuity happens.
		frames, start, delayMs, stepPct := 240, 0, 33, 96.5
		if len(os.Args) > 2 {
			frames, _ = strconv.Atoi(os.Args[2])
		}
		if len(os.Args) > 3 {
			start, _ = strconv.Atoi(os.Args[3])
		}
		if len(os.Args) > 4 {
			delayMs, _ = strconv.Atoi(os.Args[4])
		}
		if len(os.Args) > 5 {
			if v, err := strconv.ParseFloat(os.Args[5], 64); err == nil {
				stepPct = v
			}
		}
		step := stepPct / 100.0
		cx0, cy0 := -0.743643887037151, 0.131825904205330
		span := 3.2 * math.Pow(step, float64(start)) // span at the start frame
		out.WriteString("\x1b[2J\x1b[?25l")
		defer out.WriteString("\x1b[?2026l\x1b[?25h\x1b[0m") // end-sync + restore cursor on exit
		wallStart := time.Now()
		var totalComputeMs float64
		for f := 0; f < frames; f++ {
			absF := start + f
			t0 := time.Now()
			mi := 200 + absF*3
			frame := render(100, 44, cx0, cy0, span, mi)
			ms := float64(time.Since(t0).Microseconds()) / 1000.0
			totalComputeMs += ms
			// DEC 2026 synchronized output: the terminal holds the prior frame
			// until the end marker, then presents the whole batch atomically — no
			// mid-draw tearing on the ~88KB color frames (xsofy#149). Unsupported
			// terminals ignore the unknown private mode, so it's safe everywhere.
			out.WriteString("\x1b[?2026h\x1b[H") // begin-sync + home
			out.Write(frame)
			out.WriteString("\x1b[0m AOT-native  frame ")
			out.WriteString(strconv.Itoa(absF))
			out.WriteString("  span=")
			out.WriteString(strconv.FormatFloat(span, 'e', 3, 64))
			out.WriteString("  mi=")
			out.WriteString(strconv.Itoa(mi))
			out.WriteString("  compute=")
			out.WriteString(strconv.FormatFloat(ms, 'f', 1, 64))
			out.WriteString("ms  delay=")
			out.WriteString(strconv.Itoa(delayMs))
			out.WriteString("ms   \x1b[?2026l\n") // end-sync
			out.Flush()
			span *= step
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
		wall := time.Since(wallStart).Seconds()
		out.WriteString("\x1b[?2026l\x1b[?25h\x1b[0m\n") // end-sync, show cursor, reset
		out.Flush()
		// Summary to stderr: always visible in a terminal, never pollutes a piped
		// stdout (frame capture, metrics CSV, etc.).
		fmt.Fprintf(os.Stderr, "── %d frames in %.2fs = %.0f fps wall (delay=%dms) │ kernel ceiling %.0f fps (%.1f ms/frame)\n",
			frames, wall, float64(frames)/wall, delayMs,
			1000.0*float64(frames)/totalComputeMs, totalComputeMs/float64(frames))

	case "metrics":
		// Emit per-frame continuous metrics along the exact zoom trajectory,
		// so a discontinuity in the render shows up as a jump in the numbers.
		frames := 240
		if len(os.Args) > 2 {
			frames, _ = strconv.Atoi(os.Args[2])
		}
		cx0, cy0 := -0.743643887037151, 0.131825904205330
		span := 3.2
		W, H := 100, 44
		os.Stdout.WriteString("frame,span,mi,sum,interior_frac,mean,ul_detail\n")
		for f := 0; f < frames; f++ {
			mi := 200 + f*3
			spanY := span * float64(H) / float64(W) / 0.5
			var sum, interior, ulDetail int64
			for row := 0; row < H; row++ {
				cy := cy0 - spanY/2 + spanY*float64(row)/float64(H)
				for col := 0; col < W; col++ {
					cx := cx0 - span/2 + span*float64(col)/float64(W)
					it := k.Escape(nil, cx, cy, mi)
					sum += int64(it)
					if it >= mi {
						interior++
					} else if row < H/2 && col < W/2 {
						ulDetail++ // escaping (non-black) cells in the upper-left quadrant
					}
				}
			}
			n := float64(W * H)
			out.WriteString(strconv.Itoa(f) + "," + strconv.FormatFloat(span, 'e', 4, 64) + "," +
				strconv.Itoa(mi) + "," + strconv.FormatInt(sum, 10) + "," +
				strconv.FormatFloat(float64(interior)/n, 'f', 4, 64) + "," +
				strconv.FormatFloat(float64(sum)/n, 'f', 2, 64) + "," +
				strconv.FormatInt(ulDetail, 10) + "\n")
			out.Flush()
			span *= 0.965
		}

	default:
		reps := 20
		if n, err := strconv.Atoi(mode); err == nil {
			reps = n
		}
		const W, H, MI = 160, 120, 256
		var sum int64
		start := time.Now()
		for r := 0; r < reps; r++ {
			sum = 0
			for row := 0; row < H; row++ {
				cy := k.CyOf(nil, row)
				for col := 0; col < W; col++ {
					sum += int64(k.Escape(nil, k.CxOf(nil, col), cy, MI))
				}
			}
		}
		ms := float64(time.Since(start).Microseconds()) / 1000.0
		out.WriteString("native  reps=")
		out.WriteString(strconv.Itoa(reps))
		out.WriteString("  checksum=")
		out.WriteString(strconv.FormatInt(sum, 10))
		out.WriteString("  elapsed_ms=")
		out.WriteString(strconv.FormatFloat(ms, 'f', 3, 64))
		out.WriteString("\n")
	}
}
