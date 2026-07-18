module mandelbrot-aot

go 1.26

require (
	github.com/nooga/let-go v0.0.0
	golang.org/x/term v0.41.0
)

require golang.org/x/sys v0.44.0 // indirect

// Portable within the let-go-lab sibling layout: ../../let-go is the repo's
// let-go symlink. Needs let-go >= 1.12 (the ^double AOT param hints, #357/#534).
replace github.com/nooga/let-go => ../../let-go
