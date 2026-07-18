# kv4p-node

A lightweight Go toolkit for building analog FM applications with the KV4P-HT radio board.

The primary application is a portable simplex parrot that records a transmission and immediately plays it back, allowing amateur radio operators to evaluate transmitted audio, compare radios, and demonstrate equipment in the field.

The repository also contains the protocol implementation, reusable libraries, and diagnostic utilities used to communicate with the KV4P-HT over USB serial using KV4P/KISS vendor frames.

## Features

- Portable analog FM simplex parrot
- KV4P/KISS protocol implementation
- Radio configuration and state management
- Receive audio monitoring and WAV recording
- Diagnostic and protocol analysis utilities
- Automated tests for core protocol and state management

## Project Status

This project is under active development and has been successfully tested by the author on Raspberry Pi hardware using analog FM. It is suitable for experimentation and testing by experienced amateur radio operators.

Current development focuses on:

- Reliability and recovery
- Portable deployment
- Configuration improvements
- Documentation and usability

## Hardware

Developed and tested with:

- Raspberry Pi Zero 2 W
- KV4P-HT board over USB serial
- Yaesu FT5DR using analog FM
- 2-meter simplex testing (for example `146.520 MHz`)

The KV4P board normally appears as `/dev/ttyUSB0`.

On Raspberry Pi OS, the user running the tools usually needs to be a member of the `dialout` group.

## Requirements

- Go
- KV4P-HT board connected over USB
- Serial device access permissions
- Opus support for `gopkg.in/hraban/opus.v2`

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