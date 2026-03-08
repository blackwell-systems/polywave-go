package agent

import (
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// collectStream processes a streaming API response and returns the full text.
// Called internally by SendMessage; not exported.
func collectStream(stream *ssestream.Stream[anthropic.MessageStreamEventUnion]) (string, error) {
	var sb strings.Builder
	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" {
			delta := event.AsContentBlockDelta()
			if delta.Delta.Type == "text_delta" {
				sb.WriteString(delta.Delta.AsTextDelta().Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}
