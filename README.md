# kv4p-node

Go tools and protocol helpers for talking to a KV4P radio node over serial (KISS vendor frames), including HELLO parsing, desired-state control, RX monitoring, Opus decode, and simple parrot/replay flows.

## What is in this repo

- `cmd/`: runnable tools for hello/state/audio monitoring, RX config, recording, and parrot replay.
- `internal/protocol`: KV4P frame command constants and payload encoders/decoders.
- `internal/radio`: higher-level connect/configure/read/send helpers.
- `internal/transport` and `internal/client`: serial transport and KISS framing client.

## Requirements

- Go 1.24+
- A connected KV4P USB device
- Serial access permissions for your OS/user
- Opus dependency support for `gopkg.in/hraban/opus.v2` (CGO/system libs as required by your platform)

## Quick start

```bash
# 1) Wait for and print HELLO
go run ./cmd/hello

# 2) Apply receive config
go run ./cmd/rx-config --freq 146.520 --squelch 3

# 3) Monitor RX audio + state frames
go run ./cmd/monitor --freq 146.520 --squelch 3
```

If a tool says to press EN/RESET, do it once after starting that command.

## Command reference

| Command | Purpose | Example |
| --- | --- | --- |
| `cmd/hello` | Connect and print KV4P HELLO info | `go run ./cmd/hello` |
| `cmd/rx-config` | Apply receive config and print applied state | `go run ./cmd/rx-config --freq 146.520 --squelch 3` |
| `cmd/monitor` | Show RX audio packet and device-state activity | `go run ./cmd/monitor --freq 146.520 --squelch 3` |
| `cmd/decode-monitor` | Decode RX Opus packets to PCM stats | `go run ./cmd/decode-monitor --freq 146.520 --squelch 3` |
| `cmd/state-monitor` | Reconnect loop that waits for and prints HELLO | `go run ./cmd/state-monitor` |
| `cmd/state-monitor-no-reset` | Configure desired state and print state changes | `go run ./cmd/state-monitor-no-reset --freq 146.520 --squelch 8` |
| `cmd/record` | Capture RX Opus packets and save WAV when signal ends | `go run ./cmd/record --freq 146.520 --squelch 3 --output kv4p-capture.wav` |
| `cmd/capture-parrot` | Capture RX, save WAV, replay captured audio with PTT | `go run ./cmd/capture-parrot --freq 146.520 --squelch 3 --output capture.wav` |
| `cmd/parrot` | Capture RX and replay captured audio with PTT | `go run ./cmd/parrot --freq 146.520 --squelch 3` |
| `cmd/raw-dump` | Raw serial/reset dump helper (currently hardcoded `/dev/ttyUSB0`) | `go run ./cmd/raw-dump` |

## Troubleshooting

- **`KV4P USB device not found`**: reconnect device, verify VID/PID visibility, and confirm serial permissions.
- **No RX audio packets**: lower squelch, verify frequency, and confirm active signal on channel.
- **State updates but no decode output**: confirm incoming command is RX audio (`0x07`) and Opus payloads are non-empty.
- **Parrot not transmitting**: confirm TX is allowed and PTT transitions occur (`HostStateTXAllowed` / `HostStatePTTRequested`).

## Development

```bash
go test ./...
```

The project currently has no published release/install workflow; use `go run ./cmd/<tool>` for local execution.
