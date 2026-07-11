import serial
import time
import struct
import sys
import os

FEND = b'\xC0'
FESC = b'\xDB'
TFEND = b'\xDC'
TFESC = b'\xDD'

def unescape_kiss(data):
    out = bytearray()
    i = 0
    while i < len(data):
        if bytes([data[i]]) == FESC and i + 1 < len(data):
            if bytes([data[i+1]]) == TFEND:
                out.append(0xC0)
            elif bytes([data[i+1]]) == TFESC:
                out.append(0xDB)
            i += 2
        else:
            out.append(data[i])
            i += 1
    return bytes(out)

def escape_kiss(data):
    out = bytearray()
    for byte in data:
        if bytes([byte]) == FEND:
            out.extend(FESC + TFEND)
        elif bytes([byte]) == FESC:
            out.extend(FESC + TFESC)
        else:
            out.append(byte)
    return bytes(out)

def make_kv4p_vendor_frame(cmd_code, payload=b''):
    inner_payload = b"KV4P\x01" + bytes([cmd_code]) + payload
    full_packet = FEND + b'\x06' + escape_kiss(inner_payload) + FEND
    if cmd_code == 0x0D and len(full_packet) == 31:
        return FEND + b'\x06' + escape_kiss(b"KV4P" + bytes([cmd_code]) + payload) + FEND
    return full_packet

def build_desired_state(sequence, ptt_requested, freq_mhz=146.520, squelch_level=3):
    flags = 0
    flags |= (1 << 0)  # HOST_STATE_RADIO_CONFIG_VALID
    flags |= (1 << 11) # HOST_STATE_TX_ALLOWED
    flags |= (1 << 12) # HOST_STATE_ENABLE_STATUS_REPORTS
    
    if ptt_requested:
        flags |= (1 << 1) # HOST_STATE_PTT_REQUESTED
        flags |= (1 << 3) # HOST_STATE_HIGH_POWER
    else:
        flags |= (1 << 2) # HOST_STATE_RX_AUDIO_OPEN

    memory_id = -1
    bw = 0            
    freq_tx = float(freq_mhz)
    freq_rx = float(freq_mhz)
    ctcss_tx = 0      
    ctcss_rx = 0
    
    p_seq       = struct.pack("<I", sequence)  
    p_mem       = struct.pack("<i", memory_id) 
    p_flags     = struct.pack("<H", flags)     
    p_bw        = struct.pack("<B", bw)        
    p_tx_freq   = struct.pack("<f", freq_tx)   
    p_rx_freq   = struct.pack("<f", freq_rx)   
    p_tx_ctcss  = struct.pack("<B", ctcss_tx)  
    p_squelch   = struct.pack("<B", squelch_level) 
    p_rx_ctcss  = struct.pack("<B", ctcss_rx)  
    
    payload = p_seq + p_mem + p_flags + p_bw + p_tx_freq + p_rx_freq + p_tx_ctcss + p_squelch + p_rx_ctcss
    return make_kv4p_vendor_frame(0x0D, payload)

def main():
    TARGET_FREQUENCY = 146.520 
    SQUELCH_VAL = 3
    PORT = '/dev/ttyUSB0'
    
    print("======================================================")
    print(f" Starting KV4P Simplex Parrot Daemon on {TARGET_FREQUENCY} MHz")
    print("======================================================")
    
    try:
        ser = serial.Serial(PORT, 115200, timeout=0)
        
        # AUTOMATED SOFTWARE "BUTTON PRESS" FOR THE COUCH
        print("[!] Sending automated electronic reset pulse...")
        ser.dtr = False
        ser.rts = True   # Pulls chip reset line low
        time.sleep(0.1)
        ser.rts = False  # Releases it back to high, kicking off the bootloader
        time.sleep(1.0)  # Let the chip wake up naturally
        
    except Exception as e:
        print(f"CRITICAL SERIAL ERROR: {e}")
        return

    ser.reset_input_buffer()
    ser.reset_output_buffer()

    seq = 1
    audio_recording_buffer = []
    state = "IDLE" 
    last_state_packet_time = 0
    last_diag_time = 0
    last_signal_time = time.time()

    buffer = bytearray()
    print("[!] Automated sync complete. Awaiting stream data...\n")

    try:
        while True:
            current_time = time.time()

            if state != "PLAYBACK" and (current_time - last_state_packet_time > 0.4):
                seq += 1
                try:
                    ser.write(build_desired_state(seq, False, TARGET_FREQUENCY, SQUELCH_VAL))
                    ser.flush()
                except Exception:
                    pass
                last_state_packet_time = current_time

            try:
                if ser.in_waiting > 0:
                    raw_bytes = ser.read(ser.in_waiting)
                    buffer.extend(raw_bytes)
            except Exception:
                pass
                
            while b'\xC0' in buffer:
                first_fend = buffer.index(b'\xC0')
                next_fend = buffer.find(b'\xC0', first_fend + 1)
                
                if next_fend == -1:
                    break 
                
                raw_frame = buffer[first_fend:next_fend+1]
                del buffer[:next_fend+1]
                
                if len(raw_frame) < 3:
                    continue
                
                kiss_cmd = raw_frame[1]
                escaped_payload = raw_frame[2:-1]
                payload = unescape_kiss(escaped_payload)
                
                if kiss_cmd == 0x06 and len(payload) >= 6 and payload.startswith(b"KV4P"):
                    kv4p_cmd = payload[5]
                    cmd_data = payload[6:]
                    
                    if kv4p_cmd == 0x0B and len(cmd_data) >= 12:
                        flags = struct.unpack("<H", cmd_data[8:10])[0]
                        rssi = cmd_data[11]
                        
                        if current_time - last_diag_time > 2.0:
                            print(f"    [Telemetry] RSSI: {rssi} | State: {state} | Cache: {len(audio_recording_buffer)} frames")
                            last_diag_time = current_time
                        
                        signal_present = not (flags & (1 << 10))
                        is_active_signal = signal_present and (rssi > 31 and rssi != 255)
                        
                        if state == "IDLE" and is_active_signal:
                            print(f"[>>>] SQUELCH OPENED (RSSI: {rssi})! Recording...")
                            state = "RECORDING"
                            audio_recording_buffer = []
                            last_signal_time = current_time
                            
                        elif state == "RECORDING":
                            if is_active_signal:
                                last_signal_time = current_time
                            elif (current_time - last_signal_time > 1.2):
                                print(f"[<<<] CARRIER DROP: Processing {len(audio_recording_buffer)} frames...")
                                if len(audio_recording_buffer) > 15:
                                    state = "PLAYBACK"
                                else:
                                    audio_recording_buffer = []
                                    state = "IDLE"

                    elif kv4p_cmd == 0x07 and state == "RECORDING":
                        audio_recording_buffer.append(cmd_data)

            if state == "PLAYBACK":
                print("[TX] Keying PTT lines...")
                seq += 1
                ser.write(build_desired_state(seq, True, TARGET_FREQUENCY, SQUELCH_VAL))
                ser.flush()
                time.sleep(0.4) 
                
                print(f"[TX] Streaming {len(audio_recording_buffer)} frames over 0x08 Voice channel...")
                
                for frame in audio_recording_buffer:
                    if len(frame) < 4:
                        continue
                        
                    tx_packet = make_kv4p_vendor_frame(0x08, frame)
                    ser.write(tx_packet)
                    ser.flush()
                    time.sleep(0.020)
                
                print("[TX] Releasing transmitter latch...")
                seq += 1
                ser.write(build_desired_state(seq, False, TARGET_FREQUENCY, SQUELCH_VAL))
                ser.flush()
                time.sleep(0.3)
                
                audio_recording_buffer = []
                ser.reset_input_buffer()
                ser.reset_output_buffer()
                buffer.clear()
                state = "IDLE"
                print("[!] Reset complete. Resuming monitor watch loop...\n")

            try:
                time.sleep(0.001)
            except KeyboardInterrupt:
                raise

    except KeyboardInterrupt:
        print("\n\n[!] Intercepted shutdown signal. Safely releasing hardware...")
        seq += 1
        try:
            ser.write(build_desired_state(seq, False, TARGET_FREQUENCY, SQUELCH_VAL))
            ser.flush()
            time.sleep(0.2)
            ser.close()
        except:
            pass
        print("[!] Link Safe. Offline.\n")

if __name__ == "__main__":
    main()
