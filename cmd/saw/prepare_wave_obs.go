package main

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
)

// emitWaveStart emits a wave_start observability event.
// Safe to call with nil emitter.
func emitWaveStart(emitter *observability.Emitter, implSlug string, waveNum int) {
	if emitter == nil {
		return
	}
	event := observability.NewWaveStartEvent(implSlug, waveNum)
	emitter.Emit(context.Background(), event)
}
