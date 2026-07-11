import serial
import time
import os
import subprocess
import select

# --- CONFIGURATION HUB ---
SERIAL_PORT = '/dev/ttyUSB0' # Verified live data port
BAUD_RATE = 115200
TIMEOUT = 0.05
FREQUENCY = "146.520"
VOLUME = "6"

CMD_RESTART_SCRIPT = "*11"
CMD_REBOOT_PI      = "*99"

print(f"Initializing status-aware link to kv4p on {SERIAL_PORT}...")
try:
    ser = serial.Serial(
        port=SERIAL_PORT, 
        baudrate=BAUD_RATE, 
        timeout=TIMEOUT,
        rtscts=False, 
        dsrdtr=False, 
        xonxoff=False
    )
    time.sleep(1)
except Exception as e:
    print(f"CRITICAL ERROR: Cannot open port: {e}")
    os._exit(1)

# --- PHYSICAL WAKEUP ---
ser.write(b'\x00\xFF\x00\xFF')
ser.flush()
time.sleep(0.3)

ser.write(f"FREQ={FREQUENCY}\n".encode('utf-8'))
ser.write(f"VOL={VOLUME}\n".encode('utf-8'))
ser.write(b"START\n")
ser.flush()
time.sleep(0.5)
ser.reset_input_buffer()

print("--> PARROT DAEMON ALIVE: Watching hardware status beacons...")

try:
    dtmf_process = subprocess.Popen(
        ['multimon-ng', '-a', 'DTMF', '-t', 'raw', '-'],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        text=True, bufsize=1
    )
except Exception as e:
    print(f"Warning: Cannot launch DTMF monitor stack: {e}")
    dtmf_process = None

audio_buffer = bytearray()
dtmf_buffer = ""
is_recording = False
last_stream_time = time.time()
last_dtmf_time = time.time()
TEMP_FILE = "/dev/shm/field_test.opus"

while True:
    try:
        data_block = ser.read(512)
    except Exception:
        continue

    if data_block:
        # Check if the block contains the idle text beacon we saw in your cascade
        is_idle_beacon = b"KV4P" in data_block or b"CCf" in data_block
        
        if is_idle_beacon:
            # If we see the text beacon while recording, the carrier just dropped
            if is_recording and (time.time() - last_stream_time > 1.0):
                print(f"[*] Carrier dropped. Captured {len(audio_buffer)} audio bytes.")
                
                if len(audio_buffer) > 1500 and len(dtmf_buffer) == 0:
                    print("[>] Replying back over the air (Parrot Echo)...")
                    with open(TEMP_FILE, "wb") as f:
                        f.write(bytes(audio_buffer))
                    
                    # Trigger v17 PTT Transmit Sequence
                    ser.write(b"TX_START\n")
                    ser.flush()
                    time.sleep(0.4)
                    
                    with open(TEMP_FILE, "rb") as f:
                        while (chunk := f.read(64)):
                            ser.write(chunk)
                            time.sleep(0.003)
                    ser.flush()
                    time.sleep(0.2)
                    ser.write(b"TX_STOP\n")
                    ser.flush()
                    print("[+] Playback finished. Resetting standby...")
                    try: os.remove(TEMP_FILE)
                    except Exception: pass
                else:
                    print("[-] Stream reset (Signal too short or DTMF command bypassed).")
                
                is_recording = False
                audio_buffer.clear()
        else:
            # If the data block is NOT text text metrics, it is real binary voice packets
            if len(data_block) > 15:
                last_stream_time = time.time()
                
                if not is_recording:
                    print("\n[!] Voice stream detected! Recording field transmission...", flush=True)
                    is_recording = True
                    audio_buffer.clear()
                
                audio_buffer.extend(data_block)
                
                if dtmf_process and dtmf_process.stdin:
                    try:
                        dtmf_process.stdin.write(data_block.decode('latin-1'))
                        dtmf_process.stdin.flush()
                    except Exception:
                        pass

    # DTMF CAPTURE LOGIC
    if dtmf_process:
        try:
            if select.select([dtmf_process.stdout], [], [], 0.0):
                line = dtmf_process.stdout.readline().strip()
                if "DTMF:" in line:
                    digit = line.split(":")[-1].strip()
                    dtmf_buffer += digit
                    last_dtmf_time = time.time()
                    print(f"[DTMF SIGNAL]: Received digit '{digit}' | Buffer: {dtmf_buffer}", flush=True)
        except Exception:
            pass

        if len(dtmf_buffer) > 0 and (time.time() - last_dtmf_time > 4.0):
            dtmf_buffer = ""

        if CMD_RESTART_SCRIPT in dtmf_buffer:
            print("\n[OTA RESET COMMAND] -> Restarting service...", flush=True)
            if dtmf_process: dtmf_process.kill()
            ser.close()
            time.sleep(1)
            os._exit(0)

        elif CMD_REBOOT_PI in dtmf_buffer:
            print("\n[OTA REBOOT COMMAND] -> Rebooting hardware...", flush=True)
            if dtmf_process: dtmf_process.kill()
            ser.close()
            time.sleep(1)
            subprocess.run(["sudo", "reboot"])
            os._exit(0)

    time.sleep(0.01)
