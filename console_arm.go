package jukybox

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"log"
	"math"
)

type displayEvent struct {
	info DisplayInfo
}

var displayEvents = make(chan displayEvent)

func runConsole(buttonEvents chan<- Button, termEvents <-chan termbox.Event) {
	for {
		select {
		case ev := <-termEvents:
			switch ev.Type {
			case termbox.EventKey:
				if ev.Ch == 0 {
					switch ev.Key {
					case termbox.KeyArrowUp:
						buttonEvents <- UpButton
					case termbox.KeyArrowDown:
						buttonEvents <- DownButton
					case termbox.KeyArrowLeft:
						buttonEvents <- LeftButton
					case termbox.KeyArrowRight:
						buttonEvents <- RightButton
					case termbox.KeyEnter:
						buttonEvents <- CenterButton
					case termbox.KeyCtrlC:
						log.Printf("Console: Ctrl-C\n")
						buttonEvents <- PowerButton
						return
					}
				} else {
					switch ev.Ch {
					case 'a', 'A':
						buttonEvents <- AButton
					case 'b', 'B':
						buttonEvents <- BButton
					}
				}
			}
		case ev := <-displayEvents:
			time := fmt.Sprintf("%02d:%02d", int(math.Floor(ev.info.position.Minutes())), int(math.Floor(ev.info.position.Seconds()))%60)
			line1 := fmt.Sprintf("%s %s %s", ev.info.stateIcon, time, ev.info.title)
			line2 := fmt.Sprintf("    [%3d] %s", ev.info.chapterIndex, ev.info.chapterTitle)
			termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
			for i, c := range line1 {
				termbox.SetCell(i, 0, c, termbox.ColorWhite, termbox.ColorDefault)
			}
			for i, c := range line2 {
				termbox.SetCell(i, 1, c, termbox.ColorWhite, termbox.ColorDefault)
			}
			termbox.Flush()
		}
	}
}

func CreateConsole(buttonEvents chan<- Button) {
	err := termbox.Init()
	if err != nil {
		log.Fatal(err)
	}
	termEvents := make(chan termbox.Event)
	go func() {
		for {
			termEvents <- termbox.PollEvent()
		}
	}()
	go runConsole(buttonEvents, termEvents)
}

func DestroyConsole() {
	termbox.Close()
}

var previousInfo *DisplayInfo

func DrawConsole(info DisplayInfo) {
	if previousInfo == nil || *previousInfo != info {
		previousInfo = &info
		log.Printf("%#v", info)
		// displayEvents <- displayEvent{info: info}
	}
}
