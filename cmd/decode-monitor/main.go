package main

import (
	"flag"
	"log"

	"github.com/w8wot/kv4p-node/internal/audio"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/radio"
)

func main() {
	freq := flag.Float64("freq", 146.520, "Receive frequency in MHz")
	squelch := flag.Int("squelch", 3, "Squelch level 0-8")
	flag.Parse()

	decoder, err := audio.NewDecoder()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Press the KV4P EN/RESET button once.")

	r, err := radio.Connect("")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	if err := r.ConfigureReceive(float32(*freq), byte(*squelch)); err != nil {
		log.Fatal(err)
	}

	log.Printf("Decoding audio on %.3f MHz", *freq)

	var packets int
	var samples int64

	for {
		vendor, err := r.ReadVendorFrame()
		if err != nil {
			log.Fatal(err)
		}

		if vendor.Command != protocol.CommandRxAudio {
			continue
		}

		pcm, err := decoder.Decode(vendor.Payload)
		if err != nil {
			log.Printf("Decode failed: %v", err)
			continue
		}

		packets++
		samples += int64(len(pcm))

		log.Printf(
			"Decoded packet %d: %d PCM samples, total %.2f seconds",
			packets,
			len(pcm),
			float64(samples)/audio.SampleRate,
		)
	}
}
