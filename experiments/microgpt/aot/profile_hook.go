package main

// Profiling hook for the lg compile binary, which has none of its own.
// build.sh PROFILE=1 copies this into the generated module and calls
// startProfiles() first thing in main(). Same env names as let-go's
// lg_profile build: LG_CPUPROFILE=<file>, LG_MEMPROFILE=<file> (allocs
// profile, written at exit; read it with -sample_index=alloc_space).

import (
	"os"
	"runtime"
	"runtime/pprof"
)

func startProfiles() func() {
	var stops []func()
	if p := os.Getenv("LG_CPUPROFILE"); p != "" {
		if f, err := os.Create(p); err == nil && pprof.StartCPUProfile(f) == nil {
			stops = append(stops, func() { pprof.StopCPUProfile(); f.Close() })
		}
	}
	if p := os.Getenv("LG_MEMPROFILE"); p != "" {
		stops = append(stops, func() {
			if f, err := os.Create(p); err == nil {
				runtime.GC()
				pprof.Lookup("allocs").WriteTo(f, 0)
				f.Close()
			}
		})
	}
	return func() {
		for _, s := range stops {
			s()
		}
	}
}
