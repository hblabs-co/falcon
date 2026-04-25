package realtime

import (
	"fmt"

	"hblabs.co/falcon/packages/environment"
)

const (
	portKey          = "PORT"
	defaultPortValue = 8090
)

func GetParsedPort() int {
	port := environment.ParseInt(portKey, defaultPortValue)
	return port
}

func GetRawPort() string {
	port := environment.ReadOptional(portKey, fmt.Sprintf("%d", defaultPortValue))
	return port
}
