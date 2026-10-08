package microgpt

// Workaround for nooga/let-go#562: math/* calls lower to a boxed ec.Invoke
// whose vm.Value result lands in a site already typed float64. Wrapping the
// call unboxes it, so the generated Go builds.

import (
	"fmt"

	vm "github.com/nooga/let-go/pkg/vm"
)

func unbox562(v vm.Value, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	switch x := v.(type) {
	case vm.Float:
		return float64(x), nil
	case vm.Float32:
		return float64(x), nil
	case vm.Int:
		return float64(x), nil
	}
	return 0, fmt.Errorf("unbox562: unexpected %T", v)
}
