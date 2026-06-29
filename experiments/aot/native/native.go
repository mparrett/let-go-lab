// Package native is a faithful hand-written Go port of the mandelbrot demo's
// frame (compute-grid + the single-pass sixel encoder), using plain []int and
// strings.Builder -- i.e. what a let-go "native homogeneous collections / native
// array primitive" lowering WOULD emit for encode-band. It exists to measure
// the prize: how fast the encode half could be if it lowered like the compute
// half already does. Output byte length is checked against the demo's
// bytes=27695 invariant so this is the same work, not a faster-but-different one.
package native

import (
	"math"
	"strconv"
	"strings"
)

const (
	CW    = 160
	CH    = 120
	scale = 3
	NPAL  = 64
	imgW  = CW * scale // 480
	imgH  = CH * scale // 360
	bands = imgH / 6   // 60

	homeX = -0.6
	homeY = 0.0
	homeW = 3.2
)

func iq(t, phase float64) int { return int(50.0 + 50.0*math.Cos(6.2831853*(t+phase))) }

func paletteDefs() string {
	var b strings.Builder
	b.WriteString("#0;2;0;0;0")
	for i := 0; i < NPAL; i++ {
		t := float64(i) / float64(NPAL)
		b.WriteString("#")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(";2;")
		b.WriteString(strconv.Itoa(iq(t, 0.0)))
		b.WriteString(";")
		b.WriteString(strconv.Itoa(iq(t, 0.33)))
		b.WriteString(";")
		b.WriteString(strconv.Itoa(iq(t, 0.67)))
	}
	return b.String()
}

func escape(cx, cy float64, mi int) int {
	zx, zy := 0.0, 0.0
	for i := 0; i < mi; i++ {
		zx2, zy2 := zx*zx, zy*zy
		if zx2+zy2 > 4.0 {
			return i
		}
		zx, zy = zx2-zy2+cx, 2.0*(zx*zy)+cy
	}
	return mi
}

func colorIndex(iters, mi int) int {
	if iters >= mi {
		return 0
	}
	return 1 + (iters*4)%NPAL
}

func computeGrid(cx, cy, vw float64, mi int) ([]int, int) {
	grid := make([]int, CW*CH)
	cstep := vw / float64(CW)
	left := cx - vw/2.0
	top := cy - (cstep*float64(CH))/2.0
	sum := 0
	for j := 0; j < CH; j++ {
		im := top + float64(j)*cstep
		for i := 0; i < CW; i++ {
			it := escape(left+float64(i)*cstep, im, mi)
			grid[j*CW+i] = colorIndex(it, mi)
			sum += it
		}
	}
	return grid, sum
}

func rleRow(row []int, sb *strings.Builder) {
	for gx := 0; gx < len(row); {
		m := row[gx]
		run := 1
		for gx+run < len(row) && row[gx+run] == m {
			run++
		}
		ch := byte(63 + m)
		if run >= 4 {
			sb.WriteByte('!')
			sb.WriteString(strconv.Itoa(run))
			sb.WriteByte(ch)
		} else {
			for k := 0; k < run; k++ {
				sb.WriteByte(ch)
			}
		}
		gx += run
	}
}

// encodeBand: the single-pass encoder, but the per-color row buffers are native
// []int (the boxed let-go version uses transient vectors of vm.Value).
func encodeBand(grid []int, b int, sb *strings.Builder) {
	y0 := b * 6
	var rowBase [6]int
	for r := 0; r < 6; r++ {
		rowBase[r] = ((y0 + r) / scale) * CW
	}
	rows := map[int][]int{}
	for gx := 0; gx < imgW; gx++ {
		gxq := gx / scale
		for r := 0; r < 6; r++ {
			c := grid[rowBase[r]+gxq]
			tv, ok := rows[c]
			if !ok {
				tv = make([]int, imgW)
				rows[c] = tv
			}
			tv[gx] |= 1 << r
		}
	}
	for c, row := range rows {
		sb.WriteByte('#')
		sb.WriteString(strconv.Itoa(c))
		rleRow(row, sb)
		sb.WriteByte('$')
	}
	sb.WriteByte('-')
}

func encodeFrame(grid []int) string {
	var sb strings.Builder
	sb.WriteString("\x1bPq\"1;1;")
	sb.WriteString(strconv.Itoa(imgW))
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(imgH))
	sb.WriteString(paletteDefs())
	for b := 0; b < bands; b++ {
		encodeBand(grid, b, &sb)
	}
	sb.WriteString("\x1b\\")
	return sb.String()
}
