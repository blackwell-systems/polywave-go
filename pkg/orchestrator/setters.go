package orchestrator

import "github.com/blackwell-systems/scout-and-wave-go/pkg/types"

// SetParseIMPLDocFunc injects the real IMPL doc parser from pkg/protocol,
// replacing the default no-op that returns an empty IMPLDoc.
// Must be called before any call to orchestrator.New in production paths
// (e.g., from pkg/api's init() or from cmd/saw's init()).
func SetParseIMPLDocFunc(f func(path string) (*types.IMPLDoc, error)) {
	parseIMPLDocFunc = f
}
