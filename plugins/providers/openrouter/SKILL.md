# OpenRouter Provider

OpenRouter provides unified access to multiple LLM providers through a single API.

## Configuration

Set the `OPENROUTER_API_KEY` environment variable with your OpenRouter API key.

## Available Models

OpenRouter provides access to:
- OpenAI models (GPT-4o, GPT-4, GPT-3.5)
- Anthropic models (Claude 3.5, Claude 3)
- Meta models (Llama 3.1, Llama 3)
- Google models (Gemini)
- And many more

## Features

- Streaming responses
- Native tool calling support
- Fallback parsing for models without native tool support
- Usage tracking and rate limiting

## Usage

The provider is automatically configured when the API key is set. Select models using the `provider/model-name` format (e.g., `openai/gpt-4o-mini`).
