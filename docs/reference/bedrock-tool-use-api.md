# Bedrock Tool Use API Reference

Source: https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters-anthropic-claude-messages-tool-use.html

## Request Format (InvokeModel / InvokeModelWithResponseStream)

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 4096,
  "system": "System prompt here",
  "tools": [
    {
      "name": "tool_name",
      "description": "Tool description",
      "input_schema": {
        "type": "object",
        "properties": {
          "parameter_name": {
            "type": "string",
            "description": "Parameter description"
          }
        },
        "required": ["parameter_name"]
      }
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": "User message"
    }
  ]
}
```

## Response Format — Tool Use

When Claude wants to call a tool, response has `stop_reason: "tool_use"`:

```json
{
  "id": "msg_unique_id",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "I'll look up that file for you."
    },
    {
      "type": "tool_use",
      "id": "toolu_unique_id",
      "name": "tool_name",
      "input": {
        "parameter_name": "parameter_value"
      }
    }
  ],
  "stop_reason": "tool_use"
}
```

## Sending Tool Results Back

Append the assistant message, then send a user message with tool_result blocks:

```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_unique_id",
      "content": "Tool execution result as string"
    }
  ]
}
```

For errors:
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_unique_id",
  "is_error": true,
  "content": "Error message"
}
```

## Stop Reasons

- `"tool_use"` — model wants to call one or more tools. Execute them and send results back.
- `"end_turn"` — model is done. Extract text from content blocks.
- `"max_tokens"` — output was truncated.

## Streaming Format (InvokeModelWithResponseStream)

Streaming events arrive as chunks. Key event types in the chunk JSON:

- `content_block_start` with `type: "tool_use"` — start of a tool call block (contains `id` and `name`)
- `content_block_delta` with `type: "input_json_delta"` — partial JSON for tool input
- `content_block_delta` with `type: "text_delta"` — text content
- `content_block_stop` — end of a content block
- `message_delta` — contains `stop_reason`

## Key Differences from Direct Anthropic API

- Uses `anthropic_version: "bedrock-2023-05-31"` (not API version header)
- Authentication via AWS IAM (SDK handles this), not API key
- Same content block format: `tool_use` / `tool_result` / `text`
- Same `stop_reason` values
- Same multi-turn loop pattern
