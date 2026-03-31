package main

import (
	"context"
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
)

// emitWaveMerge emits a wave_merge observability event.
// Safe to call with nil emitter.
func emitWaveMerge(emitter *observability.Emitter, implSlug string, waveNum int) {
	if emitter == nil {
		return
	}
	event := observability.NewWaveMergeEvent(implSlug, waveNum)
	if res := emitter.EmitSync(context.Background(), event); res.IsFatal() {
		slog.Warn("emitWaveMerge: store write failed",
			"slug", implSlug, "wave", waveNum,
			"error", res.Errors[0].Message)
	}
}

// emitWaveFailed emits a wave_failed observability event.
// Safe to call with nil emitter.
func emitWaveFailed(emitter *observability.Emitter, implSlug string, waveNum int, reason string) {
	if emitter == nil {
		return
	}
	event := observability.NewWaveFailedEvent(implSlug, waveNum, reason)
	emitter.Emit(context.Background(), event)
}

// emitGateResult emits a gate_executed or gate_failed observability event.
// Safe to call with nil emitter.
func emitGateResult(emitter *observability.Emitter, implSlug string, waveNum int, gateType string, passed bool) {
	if emitter == nil {
		return
	}
	event := observability.NewGateExecutedEvent(implSlug, waveNum, gateType, passed)
	emitter.Emit(context.Background(), event)
}
