# let-go-lab — experiments on let-go. Run `just` for this menu. LETGO is
# overridable (defaults to ./let-go, a symlink to your lg checkout).

LETGO := env_var_or_default("LETGO", justfile_directory() / "let-go")

default:
    @just --list

# native TUI of a demo (default: mandelbrot) — needs a sixel-capable terminal
play demo="mandelbrot":
    LETGO="{{LETGO}}" scripts/play.sh {{demo}}

# build + serve a demo in the browser (HTTPS/LAN if a cert is found, else localhost HTTP)
serve demo="mandelbrot" *ARGS:
    LETGO="{{LETGO}}" scripts/serve.sh {{demo}} {{ARGS}}

# fast shell-edit loop: re-inject the shell + serve, skipping the WASM rebuild
reserve demo="mandelbrot" *ARGS:
    LETGO="{{LETGO}}" scripts/serve.sh {{demo}} --no-build {{ARGS}}

# re-fetch the pinned xterm browser assets into harness/vendor (the only
# network step; day-to-day builds inline the committed copies — see #9)
vendor-xterm:
    scripts/vendor-xterm.sh

# lg version + which let-go this points at
env:
    @echo "LETGO = {{LETGO}}"
    @"{{LETGO}}/lg" --version 2>/dev/null || echo "no lg at {{LETGO}}/lg (build it: make -C {{LETGO}} build)"

# remove built bundles
clean:
    rm -rf dist
