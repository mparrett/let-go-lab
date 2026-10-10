# cueplay — intro once, then a gapless loop

A cue player for prerendered game music. Each cue plays its intro once and
then repeats its loop with no gap at the seam. Switching cues crossfades. It
expects `NAME-intro.wav` (optional) and `NAME-loop.wav` pairs, 16-bit stereo
at 44.1 kHz.

It's a prototype for let-go's native audio binding
([nooga/let-go#255](https://github.com/nooga/let-go/issues/255)). It shows that
oto 3.5.1 handles this shape with `CGO_ENABLED=0`: one long-lived player fed
by a mixer that owns the seam. The player is kept in a struct field, because
oto closes a player once its handle becomes unreachable
([ebitengine/oto#293](https://github.com/ebitengine/oto/issues/293)).

## Run

```sh
go run . DIR CUE [CUE...]             # e.g. DIR/02-courses-intro.wav + -loop.wav
go run . -each 30s -fade 3s DIR a b   # each cue for 30s, 3s crossfades
go run . -render out.wav DIR a        # write what would play, no sound card
go test ./...                         # the seam: no missing or doubled samples
```

Plain Go plus oto. It doesn't use let-go or let-go-undertone.

Moved from let-go-undertone on 2026-10-09; the history is there.
