package main

import (
	"fmt"
	"github.com/remko/jukybox/audioplayer"
	"github.com/remko/jukybox/ffmpeg"
	"log"
	"os"
)

func play(file string) error {
	player, err := audioplayer.Create()
	if err != nil {
		return err
	}
	defer player.Stop()

	decoder, err := ffmpeg.Create(file, player.NumOutputChannels())
	if err != nil {
		return err
	}
	defer decoder.Close()

	player.Start(decoder.NumChannels(), decoder.BytesPerSample(), decoder.SampleRate())

	// skipped := false
	for {
		frame, err := decoder.ReadAudioFrame()
		if err != nil {
			return err
		}
		if frame == nil {
			break
		}
		if err := player.Write(frame.Data); err != nil {
			return err
		}
		// log.Printf("%v\n", frame.Position)
		// if !skipped && frame.Position > 8*time.Second {
		// 	decoder.Seek(40 * 60 * time.Second)
		// 	decoder.Seek((41*60 + 45) * time.Second)
		// 	skipped = true
		// }
	}
	fmt.Printf("Done!\n")
	return nil
}

func main() {
	err := play(os.Args[1])
	if err != nil {
		log.Panicf("Error: %v\n", err)
	}
}
