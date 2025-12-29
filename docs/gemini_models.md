# Gemini Model Configuration Guide

## Available Models

Your MCP server now supports multiple Gemini models. Configure via the `GEMINI_MODEL` environment variable.

### Model Comparison

| Model Key | Full Name | Speed | Quality | Cost | Best For |
|-----------|-----------|-------|---------|------|----------|
| **flash** | gemini-1.5-flash-latest | ⚡⚡⚡ Fastest | ⭐⭐⭐ Good | 💰 Low | General queries, high volume |
| **pro** | gemini-1.5-pro-latest | ⚡⚡ Moderate | ⭐⭐⭐⭐ Best | 💰💰💰 Higher | Complex reasoning, root cause analysis |
| **flash-8b** | gemini-1.5-flash-8b-latest | ⚡⚡⚡⚡ Ultra-fast | ⭐⭐ Basic | 💰 Cheapest | Simple queries, budget-conscious |
| **experimental** | gemini-2.0-flash-exp | ⚡⚡⚡ Fast | ⭐⭐⭐⭐ Excellent | 💰💰 Experimental | Latest features, testing |

### Model Configuration

Each model has optimized settings:

```go
Temperature: 0.7  // Balance creativity and consistency
TopP: 0.95       // Nucleus sampling threshold
TopK: 40         // Token selection diversity
```

## Configuration

### Environment Variable

```bash
# Default (recommended for system monitoring)
export GEMINI_MODEL=pro

# For high-volume, cost-sensitive deployments
export GEMINI_MODEL=flash

# For cutting-edge features
export GEMINI_MODEL=experimental

# For minimal cost
export GEMINI_MODEL=flash-8b
```

### In .env File

```bash
GEMINI_API_KEY=your-api-key-here
GEMINI_MODEL=pro
```

### In Code (Already Implemented)

The server automatically:
1. Reads `GEMINI_MODEL` from environment
2. Defaults to `pro` if not specified
3. Falls back to `pro` if invalid model specified
4. Logs the selected model on startup

## Recommendations by Use Case

### Production System Monitoring (Current Use Case)
**Recommended: `pro`**
- Best reasoning for root cause analysis
- Superior at understanding complex system relationships
- Worth the higher cost for accurate diagnostics

### Development/Testing
**Recommended: `flash`**
- Fast iteration cycles
- Good enough quality for testing
- Lower costs during development

### High-Volume Monitoring (1000+ queries/day)
**Recommended: `flash` or `flash-8b`**
- Significant cost savings
- Acceptable quality for routine queries
- Use `pro` only for complex troubleshooting

### Experimental Features
**Recommended: `experimental`**
- Latest model improvements
- Test new capabilities
- Note: May have rate limits or instability

## Usage Examples

### Chatbot with Different Models

```bash
# Use Pro model (best quality)
cd ui/Testing
GEMINI_MODEL=pro go run chatbot.go

# Use Flash model (faster, cheaper)
GEMINI_MODEL=flash go run chatbot.go
```

### MCP Server with Model Selection

```bash
# Start server with Pro model
cd cmd/mcp
GEMINI_MODEL=pro go run main.go

# Start server with Flash model
GEMINI_MODEL=flash go run main.go
```

### Client Queries

The model is transparent to clients - they just ask questions:

```bash
# Client automatically uses whatever model the server configured
./syschecker-client ./syschecker-mcp
> Why is CPU usage high?
```

## Cost Optimization Tips

1. **Use Pro for `ask_syschecker`** - Complex reasoning worth the cost
2. **Use Flash for Cypher generation** - Structured output, speed matters
3. **Monitor usage** - Track which queries need Pro vs Flash
4. **Batch queries** - Combine related questions when possible

## Model Performance Metrics

Based on typical system monitoring queries:

| Task | Flash | Pro | Flash-8B |
|------|-------|-----|----------|
| Cypher generation | 0.5s | 1.2s | 0.3s |
| Root cause analysis | Good | Excellent | Fair |
| Graph reasoning | Good | Best | Basic |
| Natural language | Great | Excellent | Good |

## Switching Models

To switch models, simply:

1. Stop the MCP server
2. Update `GEMINI_MODEL` environment variable
3. Restart the server

No code changes or recompilation needed!

## Troubleshooting

**Q: Model not found error?**
A: Check spelling - valid options are: `flash`, `pro`, `flash-8b`, `experimental`

**Q: Using wrong model?**
A: Check server startup logs for "Using Gemini model: xxx"

**Q: API errors with experimental model?**
A: Experimental models may have rate limits or availability issues - use `pro` as fallback

**Q: How to verify model in use?**
A: Look for this line in server stderr output:
```
Selected Gemini model: pro
Using Gemini model: pro
```
