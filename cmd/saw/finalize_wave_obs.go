package main

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
)

// emitWaveMerge emits a wave_merge observability event.
// Safe to call with nil emitter.
func emitWaveMerge(emitter *observability.Emitter, implSlug string, waveNum int) {
	if emitter == nil {
		return
	}
	event := observability.NewWaveMergeEvent(implSlug, waveNum)
	emitter.Emit(context.Background(), event)
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
