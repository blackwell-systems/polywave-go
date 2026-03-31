package main

import (
	"context"
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
)

// emitWaveStart emits a wave_start observability event.
// Safe to call with nil emitter.
func emitWaveStart(emitter *observability.Emitter, implSlug string, waveNum int) {
	if emitter == nil {
		return
	}
	event := observability.NewWaveStartEvent(implSlug, waveNum)
	if res := emitter.EmitSync(context.Background(), event); res.IsFatal() {
		slog.Warn("emitWaveStart: store write failed",
			"slug", implSlug, "wave", waveNum,
			"error", res.Errors[0].Message)
	}
}
