# pkg/notify - Extractable Notification Library

A standalone notification adapter library with zero SAW-specific dependencies.
Designed for extraction into an independent Go module.

## Quick Start

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/notify"

// Create adapters from config
slack, _ := notify.NewFromConfig("slack", map[string]string{
    "webhook_url": "https://hooks.slack.com/services/T.../B.../xxx",
})

discord, _ := notify.NewFromConfig("discord", map[string]string{
    "webhook_url": "https://discord.com/api/webhooks/123/abc",
})

telegram, _ := notify.NewFromConfig("telegram", map[string]string{
    "bot_token": "123456:ABC-DEF",
    "chat_id":   "-1001234567890",
})

// Create dispatcher and send events
dispatcher := notify.NewDispatcher(slack, discord, telegram)

event := notify.Event{
    Type:     "wave_complete",
    Severity: notify.SeverityInfo,
    Title:    "Wave 1 Complete",
    Body:     "All 3 agents succeeded.",
    Fields:   map[string]string{"wave": "1", "agents": "3"},
    Timestamp: time.Now(),
}

// Format and dispatch to all adapters
err := dispatcher.Dispatch(ctx, event, &notify.SlackFormatter{})
```

## Adapter Configuration

| Adapter  | Config Key      | Required | Description                        |
|----------|-----------------|----------|------------------------------------|
| slack    | `webhook_url`   | yes      | Slack Incoming Webhook URL         |
| discord  | `webhook_url`   | yes      | Discord Webhook URL                |
| telegram | `bot_token`     | yes      | Telegram Bot API token             |
| telegram | `chat_id`       | yes      | Telegram chat/group/channel ID     |

## Built-in Adapters

### Slack
Posts messages using Block Kit formatting with section blocks, severity-colored
sidebars, and field grids.

### Discord
Posts messages as embeds with severity-based colors (blue=info, yellow=warning,
red=error) and inline field arrays.

### Telegram
Sends messages via the Bot API `sendMessage` endpoint with Markdown formatting.
Titles are bold, fields are italicized key-value pairs.

## Custom Adapters

Implement the `Adapter` and `Formatter` interfaces:

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/result"

type Adapter interface {
    Name() string
    Send(ctx context.Context, msg Message) result.Result[SendData]
}

type Formatter interface {
    Format(event Event) Message
}
```

Register your adapter factory for config-driven instantiation:

```go
func init() {
    notify.Register("myservice", func(cfg map[string]string) (notify.Adapter, error) {
        url := cfg["url"]
        if url == "" {
            return nil, fmt.Errorf("myservice: missing url")
        }
        return &MyAdapter{url: url}, nil
    })
}
```

Then create instances via the registry:

```go
adapter, err := notify.NewFromConfig("myservice", map[string]string{"url": "https://..."})
```

## Architecture

```
Event -> Formatter -> Message -> Adapter -> External Service
                                    |
                        Dispatcher (fan-out to N adapters)
```

- **Event**: Generic notification payload (type, severity, title, body, fields)
- **Message**: Formatted output with plain text fallback and adapter-specific embeds
- **Formatter**: Transforms Event into adapter-specific Message
- **Adapter**: Delivers Message to an external service via HTTP
- **Dispatcher**: Sends to multiple adapters, collects errors
- **Registry**: Factory pattern for config-driven adapter creation
