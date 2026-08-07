package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/radio"
	"github.com/w8wot/kv4p-node/internal/transport"
)

const (
	frameDuration  = 40 * time.Millisecond
	txFramePace    = 40 * time.Millisecond
	preTXDelay     = 500 * time.Millisecond
	keyupDelay     = 250 * time.Millisecond
	postAudioDelay = 150 * time.Millisecond
	postTXCooldown = 1 * time.Second

	startRSSI = 80
	stopRSSI  = 60
	stopCount = 3

	maxPackets = 750 // 30 seconds
	minPackets = 3

	webListenAddress   = ":8080"
	webMinFrequencyMHz = 144.000
	webMaxFrequencyMHz = 148.000
)

type frequencyChangeRequest struct {
	frequency float32
	result    chan error
}

type webState struct {
	mu        sync.RWMutex
	frequency float32
	status    string
}

func (s *webState) Frequency() float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.frequency
}

func (s *webState) SetFrequency(frequency float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frequency = frequency
}

func (s *webState) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *webState) SetStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func startWebServer(
	state *webState,
	changes chan<- frequencyChangeRequest,
) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		fmt.Fprintf(w, `<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="1">
<title>KV4P Portable Parrot</title>
<style>
body {
	font-family: system-ui, sans-serif;
	max-width: 480px;
	margin: 40px auto;
	padding: 0 20px;
	background: #f5f5f5;
	color: #222;
}
main {
	background: white;
	padding: 24px;
	border-radius: 16px;
}
h1 {
	margin-top: 0;
}
.current {
	font-size: 2rem;
	font-weight: 700;
	margin: 8px 0 4px;
}
.range {
	color: #666;
	margin-bottom: 24px;
}
label {
	display: block;
	font-weight: 600;
	margin-bottom: 8px;
}
input {
	box-sizing: border-box;
	width: 100%%;
	font-size: 1.25rem;
	padding: 12px;
	margin-bottom: 12px;
}
button {
	width: 100%%;
	font-size: 1.1rem;
	padding: 12px;
	cursor: pointer;
}
</style>
</head>
<body>
<main>
<h1>KV4P Portable Parrot</h1>
<div style="margin-bottom: 20px;">
<strong>Status:</strong> %s
</div>
<div>Current frequency</div>
<div class="current">%.3f MHz</div>
<div class="range">Allowed demo range: %.3f - %.3f MHz</div>

<form method="post" action="/frequency">
<label for="frequency">New frequency (MHz)</label>
<input
	id="frequency"
	name="frequency"
	type="number"
	inputmode="decimal"
	min="%.3f"
	max="%.3f"
	step="0.001"
	value="%.3f"
	required
>
<button type="submit">Apply Frequency</button>
</form>
</main>
</body>
</html>`,
			state.Status(),
			state.Frequency(),
			webMinFrequencyMHz,
			webMaxFrequencyMHz,
			webMinFrequencyMHz,
			webMaxFrequencyMHz,
			state.Frequency(),
		)
	})

	mux.HandleFunc("/frequency", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		value, err := strconv.ParseFloat(r.FormValue("frequency"), 32)
		if err != nil {
			http.Error(w, "invalid frequency", http.StatusBadRequest)
			return
		}

		frequency := float32(value)

		if frequency < webMinFrequencyMHz ||
			frequency > webMaxFrequencyMHz {
			http.Error(
				w,
				fmt.Sprintf(
					"frequency must be between %.3f and %.3f MHz",
					webMinFrequencyMHz,
					webMaxFrequencyMHz,
				),
				http.StatusBadRequest,
			)
			return
		}

		result := make(chan error, 1)

		changes <- frequencyChangeRequest{
			frequency: frequency,
			result:    result,
		}

		if err := <-result; err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	server := &http.Server{
		Addr:    webListenAddress,
		Handler: mux,
	}

	go func() {
		log.Printf("Web interface listening on http://0.0.0.0%s", webListenAddress)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Printf("Web interface stopped: %v", err)
		}
	}()

	return server
}

func main() {
	freq := flag.Float64("freq", 146.520, "Simplex frequency in MHz")
	squelch := flag.Int("squelch", 3, "Radio squelch level 0-8")
	flag.Parse()

	portName, err := transport.FindKV4P()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Opening %s without resetting", portName)

	c, err := client.Connect(portName)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	desired := protocol.HostDesiredState{
		Sequence: uint32(time.Now().Unix()),
		MemoryID: -1,
		Flags: protocol.HostStateRadioConfigValid |
			protocol.HostStateRXAudioOpen |
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateTXAllowed |
			protocol.HostStateStatusReports,
		Bandwidth:      1,
		TXFrequencyMHz: float32(*freq),
		RXFrequencyMHz: float32(*freq),
		Squelch:        byte(*squelch),
	}

	controller := radio.NewStateController(c, desired)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 3*time.Second)
	err = controller.Apply(startupCtx)
	cancelStartup()
	if err != nil {
		log.Fatalf("Confirm startup state: %v", err)
	}

	webState := &webState{
		frequency: float32(*freq),
		status:    "Ready",
	}
	frequencyChanges := make(chan frequencyChangeRequest)

	webServer := startWebServer(webState, frequencyChanges)
	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancelShutdown()
		_ = webServer.Shutdown(shutdownCtx)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	defer func() {
		log.Println("Releasing PTT before shutdown")

		cleanupCtx, cancelCleanup := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancelCleanup()

		if err := controller.SetPTT(cleanupCtx, false); err != nil {
			log.Printf("Release PTT during shutdown: %v", err)
		}
	}()

	log.Printf(
		"Parrot ready on %.3f MHz: RX start >= %d, RX stop <= %d",
		*freq,
		startRSSI,
		stopRSSI,
	)

	var (
		receiving      bool
		packets        [][]byte
		lowRSSICount   int
		cooldownUntil  time.Time
		lastRadioError protocol.DeviceStateError
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("Interrupt received: shutting down")
			return

		case request := <-frequencyChanges:
			if receiving {
				request.result <- fmt.Errorf(
					"cannot change frequency while receiving",
				)
				continue
			}

			changeCtx, cancelChange := context.WithTimeout(
				context.Background(),
				3*time.Second,
			)

			err := controller.Update(
				changeCtx,
				func(desired *protocol.HostDesiredState) {
					desired.MemoryID = -1
					desired.TXFrequencyMHz = request.frequency
					desired.RXFrequencyMHz = request.frequency
				},
			)

			cancelChange()

			if err == nil {
				webState.SetFrequency(request.frequency)
				log.Printf(
					"Frequency changed to %.3f MHz from web interface",
					request.frequency,
				)
			}

			request.result <- err
			continue

		default:
		}

		kissFrame, err := c.ReadFrameContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Interrupt received: shutting down")
				return
			}
			log.Printf("Radio read failed: %v", err)
			return
		}

		vendor, err := protocol.DecodeVendorFrame(kissFrame)
		if err != nil {
			continue
		}

		switch vendor.Command {
		case protocol.CommandRxAudio:
			if !receiving || time.Now().Before(cooldownUntil) {
				continue
			}

			if len(packets) < maxPackets {
				packets = append(
					packets,
					append([]byte(nil), vendor.Payload...),
				)
			}

		case protocol.CommandDeviceState:
			state, err := protocol.ParseDeviceState(vendor.Payload)
			if err != nil {
				log.Printf("Bad device state: %v", err)
				continue
			}

			if state.LastError != lastRadioError {
				if state.LastError != protocol.DeviceStateErrorNone {
					log.Printf("Radio error: %s", state.LastError)
				} else if lastRadioError != protocol.DeviceStateErrorNone {
					log.Printf("Radio error cleared: %s", lastRadioError)
				}

				lastRadioError = state.LastError
			}

			if time.Now().Before(cooldownUntil) {
				continue
			}

			if !state.LatestRSSIValid {
				continue
			}

			rssi := int(state.LatestRSSI)

			if !receiving {
				if rssi >= startRSSI {
					receiving = true
					packets = nil
					lowRSSICount = 0
					webState.SetStatus("Receiving")
					log.Printf("RX started: RSSI %d", rssi)
				}
				continue
			}

			if rssi <= stopRSSI {
				lowRSSICount++
			} else {
				lowRSSICount = 0
			}

			if lowRSSICount < stopCount {
				continue
			}

			receiving = false
			lowRSSICount = 0

			if len(packets) < minPackets {
				log.Printf("Ignoring short reception: %d packets", len(packets))
				packets = nil
				webState.SetStatus("Ready")
				continue
			}

			duration := time.Duration(len(packets)) * frameDuration
			log.Printf(
				"RX ended: %d packets, approximately %s",
				len(packets),
				duration,
			)

			captured := packets
			packets = nil
			webState.SetStatus("Replaying")

			if err := replay(c, controller, captured); err != nil {
				log.Printf("Replay failed: %v", err)

				recoveryCtx, cancelRecovery := context.WithTimeout(
					context.Background(),
					3*time.Second,
				)
				_ = controller.SetPTT(recoveryCtx, false)
				cancelRecovery()
			}

			webState.SetStatus("Ready")
			cooldownUntil = time.Now().Add(postTXCooldown)
		}
	}
}

func replay(
	c *client.Client,
	controller *radio.StateController,
	packets [][]byte,
) error {
	time.Sleep(preTXDelay)

	log.Println("PTT down")
	pttDownCtx, cancelPTTDown := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	err := controller.SetPTT(pttDownCtx, true)
	cancelPTTDown()
	if err != nil {
		return err
	}

	time.Sleep(keyupDelay)

	for _, packet := range packets {
		start := time.Now()

		frame := protocol.EncodeVendorFrame(
			protocol.CommandHostTXAudio,
			packet,
		)

		if err := c.Write(frame); err != nil {
			return fmt.Errorf("send audio: %w", err)
		}

		if elapsed := time.Since(start); elapsed < txFramePace {
			time.Sleep(txFramePace - elapsed)
		}
	}

	time.Sleep(postAudioDelay)

	log.Println("PTT up")
	pttUpCtx, cancelPTTUp := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	err = controller.SetPTT(pttUpCtx, false)
	cancelPTTUp()
	if err != nil {
		return err
	}

	log.Println("Replay completed")
	return nil
}
