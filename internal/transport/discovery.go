package transport

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

var supportedUSBDevices = map[string]map[string]bool{
	"10C4": {"EA60": true},
	"1A86": {"7523": true},
}

func FindKV4P() (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", fmt.Errorf("list serial ports: %w", err)
	}

	for _, port := range ports {
		if !port.IsUSB {
			continue
		}

		vid := strings.ToUpper(port.VID)
		pid := strings.ToUpper(port.PID)

		if products, ok := supportedUSBDevices[vid]; ok && products[pid] {
			return port.Name, nil
		}
	}

	return "", fmt.Errorf("KV4P USB device not found")
}
