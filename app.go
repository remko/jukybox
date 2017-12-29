package jukybox

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type State struct {
}

func findChapter(file *MediaFile, position time.Duration) (Chapter, int, bool) {
	for i, chapter := range file.chapters {
		if position >= chapter.start && position < chapter.end {
			return chapter, i, true
		}
	}
	return Chapter{}, -1, false
}

type App struct {
	buttonEvents chan Button
	done         chan bool
	display      *Display

	player            *Player
	haveEvent         bool
	currentFileIndex  int
	currentPosition   time.Duration
	previousEvent     PlayerEvent
	previousEventTime time.Time
	mediaFiles        []*MediaFile
	mediaFilesByFile  map[string]mediaFileAndIndex
}

type mediaFileAndIndex struct {
	index int
	file  *MediaFile
}

func CreateApp() *App {
	app := App{
		done:         make(chan bool),
		buttonEvents: make(chan Button),
	}
	app.display = CreateDisplay(app.buttonEvents)
	return &app
}

func (app *App) Run() {
	go app.run()
	app.display.Run()
	log.Printf("Waiting for done signal\n")
	<-app.done
}

func (app *App) updateDisplay() {
	displayInfo := DisplayInfo{
		position:        app.getCurrentPosition(),
		duration:        -1,
		chapterIndex:    1,
		chapterDuration: -1,
	}

	if app.haveEvent && (app.previousEvent.State == Playing || app.previousEvent.State == Paused) {
		displayInfo.stateIcon = "\u25B6"
		if app.previousEvent.State == Paused {
			displayInfo.stateIcon = "\u23F8"
		}
	} else {
		displayInfo.stateIcon = "\u25A0"
	}

	mediaFile := app.currentFile()

	file := filepath.Base(mediaFile.file)
	file = file[:len(file)-len(filepath.Ext(file))]
	displayInfo.title = file
	displayInfo.chapterPosition = displayInfo.position

	displayInfo.duration = mediaFile.duration
	displayInfo.chapterDuration = mediaFile.duration

	if len(mediaFile.title) > 0 {
		displayInfo.title = mediaFile.title
	}
	if len(mediaFile.artist) > 0 {
		displayInfo.artist = mediaFile.artist
	}
	if chapter, chapterIndex, ok := findChapter(mediaFile, displayInfo.position); ok {
		displayInfo.chapterTitle = chapter.title
		displayInfo.chapterIndex = chapterIndex + 1
		displayInfo.chapterPosition = displayInfo.position - chapter.start
		displayInfo.chapterDuration = chapter.end - chapter.start
	}

	app.display.Draw(displayInfo)
	DrawConsole(displayInfo)
}

func (app *App) displayMessage(message string) {
	app.display.Draw(DisplayInfo{
		artist: message,
	})
}

func (app *App) run() {
	CreateConsole(app.buttonEvents)

	sourceDirs := []string{"/media", "test"}

	app.displayMessage("Loading media ...")

	log.Printf("Scanning dirs %v\n", sourceDirs)
	app.mediaFiles = GetMedia(sourceDirs)
	app.mediaFilesByFile = map[string]mediaFileAndIndex{}
	for i, mediaFile := range app.mediaFiles {
		log.Printf("Found file: %s (%d chapters)\n", mediaFile.file, len(mediaFile.chapters))
		log.Printf("%#v\n", mediaFile)
		app.mediaFilesByFile[mediaFile.file] = mediaFileAndIndex{
			file:  mediaFile,
			index: i,
		}
	}

	playerEvents := make(chan PlayerEvent)
	app.player = NewPlayer(playerEvents)

	app.updateDisplay()

	signalEvents := make(chan os.Signal, 2)
	signal.Notify(signalEvents, os.Interrupt, os.Kill, syscall.SIGTERM)

mainLoop:
	for {
		// log.Printf("Waiting for events\n")
		select {
		case event := <-playerEvents:
			log.Printf("Player event: %#v\n", event)
			app.haveEvent = true
			app.previousEvent = event
			app.previousEventTime = time.Now()
			app.currentPosition = event.Position
			if app.currentFile().file != event.File {
				if mediaFile, ok := app.mediaFilesByFile[event.File]; ok {
					app.currentFileIndex = mediaFile.index
				} else {
					log.Printf("File not registered: %v", event.File)
					app.currentFileIndex = 0
				}
			}
			app.updateDisplay()
		case button := <-app.buttonEvents:
			log.Printf("Button: %#v\n", button)
			switch button {
			case PowerButton:
				break mainLoop
			case DownButton:
				app.advanceFile(1, true)
			case UpButton:
				app.advanceFile(-1, true)
			case LeftButton:
				app.advanceChapter(-1)
			case RightButton:
				app.advanceChapter(1)
			case CenterButton:
				if app.haveEvent && app.previousEvent.State == Playing {
					app.player.Pause()
				} else {
					currentFile := app.currentFile()
					app.player.Play(currentFile.file, app.currentPosition, currentFile.isPassthroughSupported())
				}
			}
		// case <-time.After(1 * time.Second):
		// 	updateDisplay(state, display, mediaFilesByFile)
		case <-signalEvents:
			break mainLoop
		}
	}

	log.Printf("Stopping player ...")
	app.player.Stop()
	log.Printf("Stopping display ...")
	app.display.Stop()
	log.Printf("Stopping console ...")
	DestroyConsole()
	log.Printf("Sending done signal ...")
	app.done <- true
	log.Printf("Sent done signal ...")
}

func (app *App) advanceFile(n int, firstChapter bool) {
	mediaFileIndex := (app.currentFileIndex + len(app.mediaFiles) + n) % len(app.mediaFiles)
	mediaFile := app.mediaFiles[mediaFileIndex]
	position := time.Duration(0)
	if !firstChapter && n < 0 && len(mediaFile.chapters) > 0 {
		position = mediaFile.chapters[len(mediaFile.chapters)-1].start
	}
	app.setFile(mediaFileIndex, position)
}

func (app *App) advanceChapter(n int) {
	position := app.getCurrentPosition()
	currentFile := app.currentFile()
	if chapter, chapterIndex, ok := findChapter(currentFile, position); ok {
		nextChapter := chapterIndex + n
		if n < 0 && app.currentPosition > chapter.start+(3*time.Second) {
			nextChapter = chapterIndex
		}
		if nextChapter < 0 || nextChapter >= len(currentFile.chapters) {
			app.advanceFile(n, false)
		} else {
			app.setFile(app.currentFileIndex, currentFile.chapters[nextChapter].start)
		}
	} else {
		app.advanceFile(n, false)
	}
}

func (app *App) getCurrentPosition() time.Duration {
	if app.haveEvent && app.previousEvent.State == Playing {
		return app.previousEvent.Position + time.Since(app.previousEventTime)
	} else {
		return app.currentPosition
	}
}

func (app *App) currentFile() *MediaFile {
	return app.mediaFiles[app.currentFileIndex]
}

func (app *App) setFile(index int, position time.Duration) {
	app.currentFileIndex = index
	app.currentPosition = position
	if app.haveEvent && app.previousEvent.State == Playing {
		currentFile := app.currentFile()
		app.player.Play(currentFile.file, app.currentPosition, currentFile.isPassthroughSupported())
	}
	app.updateDisplay()
}
