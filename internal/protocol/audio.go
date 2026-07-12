package protocol

// Command 0x07 carries Opus audio in both directions.
// The meaning depends on whether the host or device sends it.
const CommandHostTXAudio byte = 0x07
