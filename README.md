# NeverForgetVPS

![Version](https://img.shields.io/github/v/tag/custom-app/NeverForgetVPS?label=version)

Library for monitoring VPS provider payment dates and sending notifications.

## Installation

```bash
go get github.com/custom-app/NeverForgetVPS@latest
```

Or specify a version:

```bash
go get github.com/custom-app/NeverForgetVPS@v1.0.0
```

## Environment Variables

Set the following environment variables before running:

```bash
# VDSina API Key (optional)
export VDSINA_API_KEY="your_vdsina_api_key"

# OneProvider API credentials (optional)
export ONEPROVIDER_API_KEY="your_oneprovider_api_key"
export ONEPROVIDER_CLIENT_KEY="your_oneprovider_client_key"
```

Or create a `.env` file (see `.env.example`) and load it:

```bash
# Using bash/zsh
export $(cat .env | xargs)

# Or using a tool like direnv or similar
```

## Example Usage

See `example/main.go` for a complete example.