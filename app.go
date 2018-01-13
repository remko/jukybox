package jukybox

import (
	"github.com/remko/jukybox/audioplayer"
	"github.com/remko/jukybox/ffmpeg"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Player state
const (
	Stopped = iota
	Playing
)

type PlayerState int

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

	mediaFiles       []*MediaFile
	mediaFilesByFile map[string]mediaFileAndIndex

	audioPlayer audioplayer.AudioPlayer
	decoder     *ffmpeg.FFmpeg
	passthrough bool

	playerState      PlayerState
	currentFileIndex int
	currentPosition  time.Duration
}

type mediaFileAndIndex struct {
	index int
	file  *MediaFile
}

func CreateApp() *App {
	audioPlayer, err := audioplayer.Create()
	if err != nil {
		log.Printf("ERROR: %v", err)
	}

	app := App{
		currentFileIndex: -1,
		done:             make(chan bool),
		buttonEvents:     make(chan Button, 2),
		audioPlayer:      audioPlayer,
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
		position:        app.currentPosition,
		duration:        -1,
		chapterIndex:    1,
		chapterDuration: -1,
	}

	switch app.playerState {
	// case Paused:
	// 	displayInfo.stateIcon = "\u23F8"
	case Playing:
		displayInfo.stateIcon = "\u25B6"
	case Stopped:
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
	} else {
		log.Printf("No chapter found %v %v", mediaFile.file, displayInfo.position)
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

	sourceDirs := []string{"/media", "./media"}

	app.displayMessage("Loading media ...")

	log.Printf("Scanning dirs %v\n", sourceDirs)
	app.mediaFiles = GetMedia(sourceDirs)
	app.mediaFilesByFile = map[string]mediaFileAndIndex{}
	for i, mediaFile := range app.mediaFiles {
		log.Printf("Found file: %s (%d chapters)\n", mediaFile.file, len(mediaFile.chapters))
		// log.Printf("%#v\n", mediaFile)
		app.mediaFilesByFile[mediaFile.file] = mediaFileAndIndex{
			file:  mediaFile,
			index: i,
		}
	}

	signalEvents := make(chan os.Signal, 2)
	signal.Notify(signalEvents, os.Interrupt, os.Kill, syscall.SIGTERM)

	app.setFile(0, time.Duration(0))

outerLoop:
	for {
		app.updateDisplay()
		switch app.playerState {
		case Stopped:
			select {
			case button := <-app.buttonEvents:
				if !app.handleButton(button) {
					break outerLoop
				}

			case <-signalEvents:
				break outerLoop
			}
		case Playing:
			select {
			case button := <-app.buttonEvents:
				if !app.handleButton(button) {
					break outerLoop
				}

			case <-signalEvents:
				break outerLoop
			default:
				var frame *ffmpeg.AudioFrame
				var err error
				if app.passthrough {
					frame, err = app.decoder.ReadAudioPacket()
				} else {
					frame, err = app.decoder.ReadAudioFrame()
				}
				if err != nil {
					log.Printf("ERROR: %v", err)
				}
				if frame == nil {
					// Song finished
					app.playerState = Stopped
					app.stopAudioPlayer()
					app.currentPosition = time.Duration(0)
					continue
				}
				// To avoid glitches while seeking
				if frame.Position > app.currentPosition {
					app.currentPosition = frame.Position
				}
				if err := app.audioPlayer.Write(frame.Data); err != nil {
					log.Printf("ERROR: %v", err)
				}
			}
		}
	}

	log.Printf("Stopping display ...")
	app.display.Stop()
	log.Printf("Stopping console ...")
	DestroyConsole()
	log.Printf("Sending done signal ...")
	app.done <- true
	log.Printf("Sent done signal ...")
}

func (app *App) handleButton(button Button) bool {
	log.Printf("Button: %#v\n", button)
	switch button {
	case PowerButton:
		return false
	case DownButton:
		app.advanceFile(1, true)
	case UpButton:
		app.advanceFile(-1, true)
	case LeftButton:
		app.advanceChapter(-1)
	case RightButton:
		app.advanceChapter(1)
	case CenterButton:
		switch app.playerState {
		case Playing:
			app.playerState = Stopped
			app.stopAudioPlayer()
		case Stopped:
			app.playerState = Playing
			app.startAudioPlayer()
		}
	}
	return true
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
	currentFile := app.currentFile()
	if chapter, chapterIndex, ok := findChapter(currentFile, app.currentPosition); ok {
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

func (app *App) currentFile() *MediaFile {
	return app.mediaFiles[app.currentFileIndex]
}

func (app *App) startAudioPlayer() {
	codec, codecProfile := app.decoder.Codec()
	app.passthrough = audioplayer.IsPassthroughSupported(codec, codecProfile, app.decoder.SampleRate())
	encoding := audioplayer.PCMEncoding
	if app.passthrough {
		encoding = codec
	}
	err := app.audioPlayer.Start(app.decoder.NumChannels(), app.decoder.BytesPerSample(), app.decoder.SampleRate(), app.decoder.IsFloatPlanar(), encoding)
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
}

func (app *App) stopAudioPlayer() {
	app.audioPlayer.Stop()
}

func (app *App) setFile(index int, position time.Duration) {
	startPlayer := false
	fileChanged := app.currentFileIndex != index
	positionChanged := (fileChanged && position != 0) || (!fileChanged && app.currentPosition != position)

	app.currentFileIndex = index
	app.currentPosition = position

	if fileChanged {
		if app.playerState == Playing {
			app.stopAudioPlayer()
			startPlayer = true
		}
		if app.decoder != nil {
			app.decoder.Close()
		}
		log.Printf("Opening %s", app.currentFile().file)
		decoder, err := ffmpeg.Create(app.currentFile().file, app.audioPlayer.NumOutputChannels())
		if err != nil {
			log.Printf("ERROR: %v", err)
		}
		app.decoder = decoder
	}

	if positionChanged {
		app.decoder.Seek(position)
	}

	if startPlayer {
		app.startAudioPlayer()
	}
}
