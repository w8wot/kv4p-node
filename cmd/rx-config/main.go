package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/w8wot/kv4p-node/internal/radio"
)

func main() {
	freq := flag.Float64("freq", 146.520, "Receive frequency in MHz")
	squelch := flag.Int("squelch", 3, "Squelch level 0-8")
	flag.Parse()

	r, err := radio.Connect("")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	if err := r.ConfigureReceive(float32(*freq), byte(*squelch)); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Receive configuration applied")
	fmt.Printf("Sequence   : %d\n", r.State.AppliedSequence)
	fmt.Printf("RX         : %.3f MHz\n", r.State.RXFrequencyMHz)
	fmt.Printf("TX         : %.3f MHz\n", r.State.TXFrequencyMHz)
	fmt.Printf("Squelch    : %d\n", r.State.Squelch)
	fmt.Printf("Flags      : 0x%04X\n", r.State.Flags)
	fmt.Printf("Last error : %d\n", r.State.LastError)
}
