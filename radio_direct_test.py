import serial
import time

# Open the connection to the USB port
# We use a 1-second timeout so it doesn't hang if the radio is quiet
ser = serial.Serial('/dev/ttyUSB0', 115200, timeout=1)

print("Forcing the ESP32 chip into a silent sleep state...")
# CP2102 hardware lines: setting dtr=True and rts=True 
# forces the ESP32 into a permanent hardware reset/sleep state.
ser.setDTR(True)
ser.setRTS(True)
time.sleep(0.5) # Give the hardware a moment to stabilize

# Flush out any leftover "invalid header" text still sitting in the buffers
ser.reset_input_buffer()

print("Sending test handshake directly to the SA818 Radio module...")
ser.write(b"AT+DMOCONNECT\r\n")

# Read the response back from the radio lines
response = ser.read(100)
print("\nRadio Reply:")
print(response)

# Clean up and release the lines
ser.close()
