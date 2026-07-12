# KV4P FM Parrot

`kv4p-parrot` is a local analog FM parrot/repeater test tool for a KV4P-HT board connected to a Raspberry Pi or other Linux host over USB.

It listens for an analog FM transmission, records the received audio packets from the KV4P board, then keys the KV4P transmitter and replays the captured audio back over RF.

The tool is intended for simplex audio testing, receive/transmit validation, and local ham radio experimentation.

## Hardware

Tested with:

- Raspberry Pi Zero 2 W
- KV4P-HT board over USB serial
- Yaesu FT5DR transmitting analog FM
- 2 meter simplex frequency, for example `146.520 MHz`

The KV4P board must appear as a USB serial device, usually `/dev/ttyUSB0`.

The user running the program must have permission to access the serial device. On Raspberry Pi OS, that usually means being in the `dialout` group.

## Build

From the repository root:

~~~bash
go build -o kv4p-parrot ./cmd/parrot
~~~

Optional diagnostic build:

~~~bash
go build -o kv4p-capture-parrot ./cmd/capture-parrot
~~~

## Run

Example:

~~~bash
./kv4p-parrot -freq 146.520 -squelch 3
~~~

The parrot will:

1. Open the KV4P serial device.
2. Configure the radio for the requested frequency.
3. Listen for received analog FM audio.
4. Capture received audio packets.
5. Key PTT.
6. Replay the received audio.
7. Release PTT.
8. Return to listening.

It continues running until stopped.

## Audio pacing

Received audio packets are treated as 40 ms frames. Replay uses 40 ms packet pacing.

This is important. Faster pacing, such as 38 ms, can make replayed audio sound rushed, distorted, or robotic. Slower pacing, such as 42 ms, can cause gaps or rough replay audio.

## Diagnostic capture mode

`kv4p-capture-parrot` works like the normal parrot, but also saves the received audio to a WAV file before replaying it.

Example:

~~~bash
./kv4p-capture-parrot -freq 146.520 -squelch 3 -output live-capture.wav
~~~

By default, the same output filename is overwritten each time. This is useful for testing the most recent capture.

## Firmware compatibility

Some KV4P firmware versions report 24-byte device state payloads, while newer protocol definitions expect 26-byte payloads with RSSI information.

The parser accepts both forms:

- 24-byte legacy state packets
- 26-byte state packets with `LastError` and `LatestRSSI`

RSSI-triggered parrot operation requires state packets that include valid RSSI.

## Stopping safely

Press `Ctrl+C` to stop the parrot.

The program attempts to clear PTT and reopen RX audio before exiting, so the KV4P board should not remain keyed after interruption.

If the radio ever remains keyed after a forced stop, press the KV4P board's EN/RESET button.

## Notes

This tool does not require Internet access once built. It only needs the local host, the KV4P board, USB serial access, RF hardware, and power.

Internet access is only needed for tasks such as cloning the repository, installing dependencies, updating firmware, or transferring saved captures by network.
