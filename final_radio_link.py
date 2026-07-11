import serial
import time
import sys

# Connect to the USB serial port at standard speed
ser = serial.Serial('/dev/ttyUSB0', 115200, timeout=1)

print("Forcing MicroPython into a clean execution state...")
ser.write(b"\x03\x03") # Ctrl+C to break running code
time.sleep(0.2)
ser.write(b"\x01") # Enter Raw REPL mode
time.sleep(0.2)
ser.reset_input_buffer()

# This script runs entirely in software on the ESP32 to bypass the UART firmware bug
passthrough_script = """
import machine
import time

# Initialize control lines
p19 = machine.Pin(19, machine.Pin.OUT)
p18 = machine.Pin(18, machine.Pin.OUT)

# Ensure the SA818 module is powered up and out of reset
p19.value(1)
p18.value(1)

# Configure the serial lines as raw digital I/O
tx_pin = machine.Pin(17, machine.Pin.OUT)
rx_pin = machine.Pin(16, machine.Pin.IN)

# Default TX line to High (idle state for serial communication)
tx_pin.value(1)

# Use UART0 for communication with the Raspberry Pi
u0 = machine.UART(0, baudrate=115200)

print("BITBANG_BRIDGE_ACTIVE")

# Pre-calculate bit timing delay for 9600 baud (~104 microseconds)
bit_delay = 104

def tx_byte(b):
    # Start bit (Low)
    tx_pin.value(0)
    machine.time_pulse_us(tx_pin, 0, bit_delay) # Use hardware clock for precise delay
    
    # 8 Data bits
    for i in range(8):
        tx_pin.value((b >> i) & 1)
        machine.time_pulse_us(tx_pin, 0, bit_delay)
        
    # Stop bit (High)
    tx_pin.value(1)
    machine.time_pulse_us(tx_pin, 0, bit_delay)

while True:
    if u0.any():
        data = u0.read(1)
        for byte in data:
            tx_byte(byte)
"""

print("Deploying software-defined routing plane...")
ser.write(passthrough_script.encode('utf-8') + b"\x04")
time.sleep(1.0)

# Read initialization status
status = ser.read(ser.in_waiting or 100)
print(f"Status from Chip: {status}")

print("\nSending connection handshake string (AT+DMOCONNECT)...")
ser.write(b"AT+DMOCONNECT\r\n")
time.sleep(0.5)

response = ser.read(ser.in_waiting or 100)
print("\nRadio Initial Response:")
print(response)

print("\nEntering Live Interactive Terminal Mode. Press Ctrl+C to exit.\n")
try:
    while True:
        if ser.in_waiting:
            sys.stdout.write(ser.read(ser.in_waiting).decode('utf-8', errors='ignore'))
            sys.stdout.flush()
except KeyboardInterrupt:
    print("\nClosing Link.")
    ser.close()
