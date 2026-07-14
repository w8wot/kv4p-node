# kv4p-node

Go tools and protocol helpers for talking to a KV4P-HT radio board over USB serial using KV4P/KISS vendor frames.

This repository includes tools for:

- detecting and reading KV4P HELLO information
- applying desired radio state
- monitoring device-state and RX audio frames
- recording received audio to WAV
- running an analog FM parrot capture/replay tool

## Hardware

Developed and tested with:

- Raspberry Pi Zero 2 W
- KV4P-HT board over USB serial
- Yaesu FT5DR using analog FM
- 2 meter simplex testing, for example `146.520 MHz`

The KV4P board normally appears as `/dev/ttyUSB0`.

On Raspberry Pi OS, the user running the tools usually needs to be in the `dialout` group.

## Requirements

- Go
- KV4P-HT board connected over USB
- Serial device access permissions
- Opus support for Go package `gopkg.in/hraban/opus.v2`

## Build

Build the main parrot tool:

~~~bash
go build -o kv4p-parrot ./cmd/parrot
~~~

Build the diagnostic capture parrot:

~~~bash
go build -o kv4p-capture-parrot ./cmd/capture-parrot
~~~

Run all tests:

~~~bash
go test ./...
~~~

## Common commands

Read KV4P HELLO information:

~~~bash
go run ./cmd/hello
~~~

Configure receive:

~~~bash
go run ./cmd/rx-config --freq 146.520 --squelch 3
~~~

Monitor RX audio and state frames:

~~~bash
go run ./cmd/monitor --freq 146.520 --squelch 3
~~~

Record received audio to WAV:

~~~bash
go run ./cmd/record --freq 146.520 --squelch 3 --output kv4p-capture.wav
~~~

Run the FM parrot:

~~~bash
./kv4p-parrot -freq 146.520 -squelch 3
~~~

Run the diagnostic capture parrot:

~~~bash
./kv4p-capture-parrot -freq 146.520 -squelch 3 -output live-capture.wav
~~~

## FM parrot

The parrot listens for analog FM audio, stores the received audio packets, keys PTT, replays the captured audio, then returns to listening.

Replay uses 40 ms packet pacing to match the received Opus frame duration.

Detailed parrot notes are in:

- [`docs/parrot.md`](docs/parrot.md)

## Offline use

Once built, the parrot does not require Internet access.

It only needs:

- the local host
- the KV4P board
- USB serial access
- RF hardware
- power

Internet is only needed for cloning, updating, installing dependencies, or transferring saved captures over the network.

## Development status

This is experimental ham radio tooling for KV4P-HT development and testing.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

## Disclaimer

This is an independent, community-developed project and is not affiliated with
or endorsed by the KV4P-HT project or its maintainers.

## Close-Range RF Testing

During development we observed that over-the-air bench testing with a transmitting radio only a few feet from the KV4P node can produce degraded or "picketing" audio. In our testing, increasing separation between the transmitting radio and the node, or reducing RF coupling, consistently improved the received audio.

For more representative bench testing:

- Maintain reasonable physical separation between the transmitting radio and the KV4P node.
- Reduce transmit power when practical.
- If close-range testing is necessary, reducing RF coupling, for example by using a stubby antenna, may produce more representative results.
- For the most repeatable bench testing, use appropriate RF attenuation or a dummy load/coupler instead of over-the-air testing.

These recommendations are intended for bench testing. Always perform a final verification under normal operating conditions representative of your intended deployment.
