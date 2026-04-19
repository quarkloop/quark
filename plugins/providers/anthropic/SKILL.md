# Anthropic Provider

Direct access to Anthropic's API for Claude models.

## Configuration

Set the `ANTHROPIC_API_KEY` environment variable with your Anthropic API key.

## Available Models

- `claude-sonnet-4-20250514` - Claude Sonnet 4 (default)
- `claude-opus-4-20250514` - Claude Opus 4, most capable
- `claude-3-5-sonnet-20241022` - Claude 3.5 Sonnet
- `claude-3-opus-20240229` - Claude 3 Opus
- `claude-3-haiku-20240307` - Claude 3 Haiku, fast and efficient

## Features

- Streaming responses
- Native tool/function calling
- 200K context window
- Extended thinking support

## Usage

The provider is automatically configured when the API key is set. Models are selected by their ID.

## Note

This provider uses Anthropic's Messages API which has a different format than OpenAI's. The plugin automatically converts between formats internally.
