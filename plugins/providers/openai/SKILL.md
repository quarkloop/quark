# OpenAI Provider

Direct access to OpenAI's API for GPT models.

## Configuration

Set the `OPENAI_API_KEY` environment variable with your OpenAI API key.

## Available Models

- `gpt-4o` - Most capable model, 128K context (default)
- `gpt-4o-mini` - Smaller, faster GPT-4o variant
- `gpt-4-turbo` - GPT-4 with vision and improved performance
- `gpt-4` - Original GPT-4, 8K context
- `gpt-3.5-turbo` - Fast and cost-effective

## Features

- Streaming responses
- Native tool/function calling
- Vision support (GPT-4 models)
- JSON mode support

## Usage

The provider is automatically configured when the API key is set. Models are selected by their ID (e.g., `gpt-4o`).
