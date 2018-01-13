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

	codec, codecProfile := decoder.Codec()
	fmt.Printf("Codec: %v (%v)\n", codec, codecProfile)
	passthrough := audioplayer.IsPassthroughSupported(codec, codecProfile, decoder.SampleRate())
	encoding := audioplayer.PCMEncoding
	if passthrough {
		encoding = codec
	}

	player.Start(decoder.NumChannels(), decoder.BytesPerSample(), decoder.SampleRate(), decoder.IsFloatPlanar(), encoding)

	for {
		var frame *ffmpeg.AudioFrame
		var err error
		if passthrough {
			frame, err = decoder.ReadAudioPacket()
		} else {
			frame, err = decoder.ReadAudioFrame()
		}
		if err != nil {
			return err
		}
		if frame == nil {
			break
		}
		if err := player.Write(frame.Data); err != nil {
			return err
		}
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
