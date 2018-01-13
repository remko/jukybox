package main

import (
	"fmt"
	"github.com/remko/jukybox/ffmpeg"
	"log"
	"os"
)

func convert(file string, out string) error {
	decoder, err := ffmpeg.Create(file, 8)
	if err != nil {
		return err
	}
	defer decoder.Close()

	f, err := os.Create(out)
	defer f.Close()
	for {
		frame, err := decoder.ReadAudioFrame()
		if err != nil {
			return err
		}
		if frame == nil {
			break
		}
		f.Write(frame.Data)
	}
	fmt.Printf("Done!\n")
	return nil
}

func main() {
	err := convert(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
