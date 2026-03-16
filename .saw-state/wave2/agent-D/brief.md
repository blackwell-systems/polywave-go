# Agent D Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-bedrock-structured-output.yaml

## Files Owned

- `pkg/engine/runner.go`
- `pkg/engine/runner_structured_test.go`


## Task

## What to Implement

Integrate Bedrock structured output into the engine layer so that
`RunScout` can use the Bedrock backend with structured output when
the model has a `bedrock:` prefix and `UseStructuredOutput` is true.

### Changes to `pkg/engine/runner.go`:

1. Add `runScoutStructuredBedrock()` function:
   ```go
   func runScoutStructuredBedrock(ctx context.Context, opts RunScoutOpts, prompt string, onChunk func(string)) (*protocol.IMPLManifest, error)
   ```
   - Similar to existing `runScoutStructured()` but uses Bedrock client
   - Import `bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"`
   - Parse provider prefix from opts.ScoutModel to get bare model
   - Expand Bedrock model ID using `orchestrator.expandBedrockModelID` (note: this
     is unexported, so either call `orchestrator.NewBackendFromModel` and type-assert,
     or duplicate the expansion logic. Preferred: create the Bedrock client directly
     with `bedrockbackend.New(backend.Config{Model: fullModelID})`)
   - Call `.WithOutputConfig(schema)` on the Bedrock client
   - Use `client.Run()` (non-streaming, since structured output produces full JSON)
   - Unmarshal JSON response into `protocol.IMPLManifest`
   - Save via `protocol.Save()`

2. Modify `RunScout()` to route to Bedrock structured output:
   - In the `if opts.UseStructuredOutput` branch, check if ScoutModel has `bedrock:` prefix
   - If bedrock prefix: call `runScoutStructuredBedrock()`
   - Else: call existing `runScoutStructured()` (API backend path)

   ```go
   if opts.UseStructuredOutput {
     provider, _ := parseProviderPrefix(opts.ScoutModel)
     if provider == "bedrock" {
       _, execErr = runScoutStructuredBedrock(ctx, opts, prompt, onChunk)
     } else {
       _, execErr = runScoutStructured(ctx, opts, prompt, onChunk)
     }
   }
   ```

   Note: `parseProviderPrefix` is in `pkg/orchestrator`. Either import and use
   a simple strings.SplitN locally, or add a helper. Simplest: inline
   `strings.HasPrefix(opts.ScoutModel, "bedrock:")`.

## Interfaces to Call
- `bedrockbackend.New(backend.Config{...})` - creates Bedrock client
- `client.WithOutputConfig(schema)` - enables structured output (Agent C implements)
- `protocol.GenerateScoutSchema()` - existing, generates IMPL doc JSON schema
- `protocol.Save()` - existing, saves manifest as YAML
- `json.Unmarshal()` - standard library

## Tests to Write

In `runner_structured_test.go` (new file):
1. TestRunScoutStructuredBedrock_NilModel - verify error when model is empty
2. TestRunScout_BedrockStructuredRouting - verify that RunScout with
   ScoutModel="bedrock:claude-sonnet-4-5" and UseStructuredOutput=true
   routes to the Bedrock path (use a mock or verify the function is called)
3. TestRunScout_APIStructuredRouting - verify that RunScout with
   ScoutModel="claude-sonnet-4-5" and UseStructuredOutput=true still
   routes to the existing API path

## Verification Gate
```bash
go build ./...
go vet ./...
go test ./pkg/engine/... -run TestRunScoutStructured -v
```

## Constraints
- Do NOT modify `runScoutStructured()` - it is the existing API backend path
- Do NOT modify `runScoutAutomation()` or `readContextMD()` - unrelated functions
- The Bedrock structured output path should NOT use streaming - use `client.Run()`
  since structured output returns complete JSON (not streamed chunks)
- If onChunk is provided, call it with the full JSON response as a single chunk
  after receiving it (for SSE progress visibility)
- Handle the case where AWS credentials are not configured: return a clear error
  message suggesting `aws configure` or env vars



## Interface Contracts

### Client.WithOutputConfig

Adds structured output JSON schema to the Bedrock client, mirroring the
api.Client.WithOutputConfig pattern. When set, Converse/ConverseStream
requests include OutputConfig with the JSON schema constraint.


```
func (c *Client) WithOutputConfig(schema map[string]any) *Client

```

### Client.RunConverse (non-streaming)

Internal method using the Converse API for non-streaming requests.
Replaces the current InvokeModel-based Run method. Supports structured
output via OutputConfig when outputSchema is set.


```
func (c *Client) Run(ctx context.Context, systemPrompt, userPrompt, workDir string) (string, error)
// When c.outputSchema != nil, builds:
//   OutputConfig: &types.OutputConfig{
//     TextFormat: &types.OutputFormat{
//       Type: types.OutputFormatTypeJsonSchema,
//       Structure: &types.OutputFormatStructureMemberJsonSchema{
//         Value: types.JsonSchemaDefinition{
//           Schema: aws.String(jsonSchemaString),
//           Name:   aws.String("structured_output"),
//         },
//       },
//     },
//   }

```

### Client.RunStreamingWithTools (Converse API)

Migrates the tool-use loop from InvokeModelWithResponseStream to
ConverseStream API. Uses types.Message, types.ContentBlock, and
types.ToolConfiguration instead of raw JSON maps. Supports structured
output via OutputConfig on the final turn.


```
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error)
// Uses ConverseStream with:
//   Messages []types.Message (typed, not map[string]interface{})
//   ToolConfig *types.ToolConfiguration (SDK tool specs)
//   System []types.SystemContentBlock
//   InferenceConfig *types.InferenceConfiguration{MaxTokens: aws.Int32(n)}

```

### buildConverseTools

Converts Workshop tools to Bedrock Converse API ToolConfiguration format.
Replaces buildToolsJSON for the Converse path.


```
func buildConverseTools(workshop tools.Workshop) *types.ToolConfiguration
// Returns &types.ToolConfiguration{
//   Tools: []types.Tool{
//     &types.ToolMemberToolSpec{Value: types.ToolSpecification{
//       Name: aws.String(name),
//       Description: aws.String(desc),
//       InputSchema: &types.ToolInputSchemaMemberJson{Value: document},
//     }},
//   },
// }

```

### GenerateCompletionReportSchema

Generates a JSON Schema for CompletionReport structured output,
mirroring GenerateScoutSchema but for agent completion reports.


```
func GenerateCompletionReportSchema() (map[string]any, error)

```

### runScoutStructuredBedrock

Engine-layer function that runs Scout via Bedrock backend with structured
output, parallel to the existing runScoutStructured (API backend).
Uses WithOutputConfig on the Bedrock client.


```
func runScoutStructuredBedrock(ctx context.Context, opts RunScoutOpts, prompt string, onChunk func(string)) (*protocol.IMPLManifest, error)

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: false)
  Check for common Go mistakes
- **test**: `go test ./pkg/agent/backend/bedrock/... ./pkg/protocol/... ./pkg/engine/...` (required: true)
  Focused test on changed packages

