package jukybox

import (
	"strconv"
	"time"
)

func playCommand(file string, position time.Duration, passthrough bool, dbusName string) []string {
	result := []string{
		"/usr/bin/omxplayer.bin",
		"--dbus_name", dbusName,
		"--no-osd", "--no-keys",
		"--layout", "5.1",
		"-o", "hdmi",
		"--pos", strconv.Itoa(int(int64(position) / 1e9)), file}
	if passthrough {
		result = append(result, "-p")
	}
	return result
}
