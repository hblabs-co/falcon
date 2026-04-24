package realtime

import (
	"fmt"

	environment "hblabs.co/falcon/common/environment"
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
