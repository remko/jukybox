package jukybox

import (
	"github.com/chbmuc/lirc"
)

func CreateLIRCRemote(buttonEvents chan<- Button) {
	ir, err := lirc.Init("/var/run/lirc/lircd")
	if err != nil {
		panic(err)
	}
	prevEvent := lirc.Event{}
	ir.Handle("", "", func(event lirc.Event) {
		if prevEvent.Button == event.Button && prevEvent.Remote == event.Remote && event.Repeat > prevEvent.Repeat {
			return
		}
		switch event.Button {
		case "KEY_KPPLUS":
			buttonEvents <- PreviousAlbumButton
		case "KEY_KPMINUS":
			buttonEvents <- NextAlbumButton
		case "KEY_REWIND":
			buttonEvents <- PreviousTrackButton
		case "KEY_FASTFORWARD":
			buttonEvents <- NextTrackButton
		case "KEY_PLAY":
			buttonEvents <- PlayPauseButton
		case "KEY_MENU":
			buttonEvents <- AButton
		}
		prevEvent = event
	})
	go ir.Run()
}

func CreateRemote(buttonEvents chan<- Button) {
	CreateLIRCRemote(buttonEvents)
}
