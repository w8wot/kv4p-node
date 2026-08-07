# kv4p-node

A portable Go implementation of the KV4P radio protocol and related tooling.

The primary application is currently a portable analog FM simplex parrot that records a transmission and immediately plays it back, allowing amateur radio operators to evaluate transmitted audio, compare radios, and demonstrate equipment in the field.

The repository also contains reusable protocol, radio, audio, and diagnostic libraries used to communicate with KV4P-HT hardware.

Linux/Raspberry Pi is the current reference implementation and primary development platform. The project is designed to keep its core radio behavior portable so future implementations on other supported operating systems can reuse the same protocol and application logic.

## Features

- Portable analog FM simplex parrot
- KV4P/KISS protocol implementation
- Radio configuration and state management
- Receive audio monitoring and WAV recording
- Diagnostic and protocol analysis utilities
- Automated tests for core protocol and state management

## Architecture

The project is organized around a portable core with platform-specific integration kept separate whenever practical.

### Portable core

- KV4P/KISS protocol implementation
- Radio state management
- Audio framing and processing
- Application logic
- Error handling

### Transport layer

Current:
- USB serial

Future:
- Bluetooth Low Energy (BLE)
- Additional transports where appropriate

### Platform integration

Linux/Raspberry Pi is the current reference implementation.

Platform-specific concerns such as device discovery, service management, and user interfaces should remain outside the reusable core whenever practical.

## Project Status

This project is under active development.

Linux/Raspberry Pi currently serves as the reference implementation and primary test platform. The project architecture is intended to remain portable so additional supported operating systems can be added without rewriting the core radio behavior.

It is suitable for experimentation and testing by experienced amateur radio operators.

Current development focuses on:

- Reliability and recovery
- Portable deployment
- Configuration improvements
- Documentation and usability

## Hardware

Reference test platform:

- Raspberry Pi Zero 2 W
- KV4P-HT board over USB serial
- Yaesu FT5DR using analog FM
- 2-meter simplex testing (for example `146.520 MHz`)

The KV4P board normally appears as `/dev/ttyUSB0`.

On Raspberry Pi OS, the user running the tools usually needs to be a member of the `dialout` group.

## Requirements

Core requirements:

- Go
- KV4P-HT compatible hardware
- Opus support for `gopkg.in/hraban/opus.v2`

Reference transport:

- KV4P-HT board connected over USB
- Serial device access permissions

## Build

Build the main parrot application:

```bash
go build -o kv4p-parrot ./cmd/parrot
```

Build the capture-and-record diagnostic tool:

```bash
go build -o kv4p-capture-parrot ./cmd/capture-parrot
```

Run all tests:

```bash
go test ./...
```

## Supported Applications and Commands

### Simplex Parrot

Run the primary analog FM parrot application:

```bash
go run ./cmd/parrot
```

### Diagnostic Utilities

Read KV4P HELLO information:

```bash
go run ./cmd/hello
```

Configure receive:

```bash
go run ./cmd/rx-config --freq 146.520 --squelch 3
```

Monitor RX audio and state frames:

```bash
go run ./cmd/monitor --freq 146.520 --squelch 3
```

Record received audio to a WAV file:

```bash
go run ./cmd/record --freq 146.520 --squelch 3 --output kv4p-capture.wav
```

## License

See the LICENSE file for licensing information.