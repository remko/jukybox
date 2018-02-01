// +build darwin

package jukybox

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
	"image"
	"image/draw"
	"os"
)

type Display struct {
	buttonChannel chan<- Button
	displayEvents chan DisplayInfo
	window        wde.Window
	drawer        *DisplayDrawer
}

func CreateDisplay(buttonChannel chan<- Button) *Display {
	dw, err := wde.NewWindow(DISPLAY_WIDTH, DISPLAY_HEIGHT)
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(-1)
	}
	dw.SetTitle("JukyBox")
	dw.Show()

	return &Display{
		buttonChannel: buttonChannel,
		displayEvents: make(chan DisplayInfo),
		window:        dw,
		drawer:        CreateDisplayDrawer(),
	}
}

func (d *Display) Run() {
	go d.run()
	wde.Run()
}

func (d *Display) run() {
	events := d.window.EventChan()
	s := d.window.Screen()
	draw.Draw(s, s.Bounds(), image.Black, image.ZP, draw.Src)
	for {
		select {
		case event := <-events:
			switch event := event.(type) {
			case wde.KeyTypedEvent:
				// fmt.Printf("typed key %v, glyph %v, chord %v\n", event.Key, event.Glyph, event.Chord)
				switch event.Key {
				case wde.KeyDownArrow:
					d.buttonChannel <- NextAlbumButton
				case wde.KeyUpArrow:
					d.buttonChannel <- PreviousAlbumButton
				case wde.KeyLeftArrow:
					d.buttonChannel <- PreviousTrackButton
				case wde.KeyRightArrow:
					d.buttonChannel <- NextTrackButton
				case wde.KeyReturn:
					d.buttonChannel <- PlayPauseButton
				case wde.KeyA:
					d.buttonChannel <- AButton
				case wde.KeyB:
					d.buttonChannel <- BButton
				}
				// case wde.ResizeEvent:
				// 	d.window.SetSize(DISPLAY_WIDTH, DISPLAY_HEIGHT)
			case wde.CloseEvent:
				d.buttonChannel <- PowerButton
			}
		case event := <-d.displayEvents:
			d.drawer.Draw(d, event)
		}
	}
}

func (d *Display) Stop() {
	d.window.Close()
	wde.Stop()
}

func (d *Display) Draw(info DisplayInfo) {
	d.displayEvents <- info
}

// Can only be called from the DisplayDrawer
func (d *Display) Flush() {
	d.window.FlushImage()
}

// Can only be called from the DisplayDrawer
func (d *Display) Image() draw.Image {
	return d.window.Screen()
}
