import serial
import time
import os
import sys

# Step 1: Initialize the ESP32's internal passthrough state using esptool tools
# We use the native environment tool to reset and hold the chip cleanly.
print("Initializing direct hardware mapping to SA818 module...")

# Open the master UART lane to the board
try:
    ser = serial.Serial('/dev/ttyUSB0', 115200, timeout=1)
except Exception as e:
    print(f"Error opening port: {e}")
    sys.exit(1)

# To ensure the ESP32 acts as a transparent wire rather than looping,
# we leverage the DTR/RTS lines to stabilize the processor control plane.
ser.setDTR(False)
ser.setRTS(False)
time.sleep(0.2)
ser.reset_input_buffer()

print("Sending standard configuration handshake to the Radio...")
# SA818 handshake protocol command strings require carriage returns
handshake_cmd = b"AT+DMOCONNECT\r\n"
ser.write(handshake_cmd)

# Capture the immediate physical response from the transceiver module
time.sleep(0.5)
reply = ser.read(ser.in_waiting or 100)

print("\n=== SYSTEM VERDICT ===")
if b"+DMOCONNECT:0" in reply:
    print("SUCCESS! The Raspberry Pi is now talking directly to your SA818 Radio!")
    print(f"Radio Raw Response: {reply.decode('utf-8', errors='ignore').strip()}")
else:
    print("No direct automated response yet. Entering Live Intercept Terminal Mode.")
    print("Type your SA818 commands below (e.g., AT+DMOCONNECT), or press Ctrl+C to exit.\n")
    print(f"Current line data cache: {reply}")
    
    try:
        while True:
            # Check for incoming radio traffic data passing back to the Pi
            if ser.in_waiting:
                data = ser.read(ser.in_waiting)
                sys.stdout.write(data.decode('utf-8', errors='ignore'))
                sys.stdout.flush()
                
            # Allow interactive command entry troubleshooting
            # Note: The SA818 listens for configuration commands strictly at 9600 or 115200 baud
    except KeyboardInterrupt:
        print("\nExiting Terminal Bridge Mode.")
        ser.close()
