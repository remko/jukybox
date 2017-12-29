// +build !arm

package jukybox

import (
	"strconv"
	"time"
)

func playCommand(file string, position time.Duration, passthrough bool, dbusName string) []string {
	return []string{"/usr/local/bin/fake-mpris-player", "-name", dbusName, "-duration", strconv.Itoa(60 * 9000), "-position", strconv.Itoa(int(int64(position) / 1e9))}
}
