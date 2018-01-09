// +build !arm

package audioplayer

/*
#cgo pkg-config: portaudio-2.0
#include <portaudio.h>
*/
import "C"

import (
	"fmt"
	"log"
	"sync"
	"unsafe"
)

var initializePA sync.Once

type PortAudioPlayer struct {
	device         C.PaDeviceIndex
	stream         unsafe.Pointer
	numChannels    int
	bytesPerSample int
}

func Create() (*PortAudioPlayer, error) {
	initializePA.Do(func() {
		C.Pa_Initialize()
		for i := 0; i < int(C.Pa_GetDeviceCount()); i += 1 {
			deviceInfo := C.Pa_GetDeviceInfo(C.PaDeviceIndex(i))
			log.Printf("Detected device: %v (%d channels)", C.GoString(deviceInfo.name), int(deviceInfo.maxOutputChannels))
		}
		//defer C.Pa_Terminate()
	})
	device := C.Pa_GetDefaultOutputDevice()
	if device == C.paNoDevice {
		return nil, fmt.Errorf("No output device")
	}

	deviceInfo := C.Pa_GetDeviceInfo(device)
	log.Printf("Selected Device: %s (%d channels)", C.GoString(deviceInfo.name), int(deviceInfo.maxOutputChannels))

	return &PortAudioPlayer{
		device: device,
	}, nil
}

func (p *PortAudioPlayer) Stop() {
	if p.stream != nil {
		C.Pa_StopStream(p.stream)
		C.Pa_CloseStream(p.stream)
	}
}

func (p *PortAudioPlayer) Start(numChannels int, bytesPerSample int, sampleRate int) error {
	p.numChannels = numChannels
	p.bytesPerSample = bytesPerSample
	outputParameters := C.PaStreamParameters{
		device:                    p.device,
		channelCount:              C.int(numChannels),
		suggestedLatency:          0.050,
		hostApiSpecificStreamInfo: nil,
	}
	if bytesPerSample == 2 {
		outputParameters.sampleFormat = C.paInt16
	} else {
		outputParameters.sampleFormat = C.paInt32
	}
	if err := C.Pa_OpenStream(&p.stream, nil, &outputParameters, C.double(sampleRate), 0, C.paClipOff, nil, nil); err != C.paNoError {
		return paError(err)
	}
	if err := C.Pa_StartStream(p.stream); err != C.paNoError {
		return paError(err)
	}
	return nil
}

func (p *PortAudioPlayer) NumOutputChannels() int {
	return int(C.Pa_GetDeviceInfo(p.device).maxOutputChannels)
}

func (p *PortAudioPlayer) Write(data []byte) error {
	nbSamples := len(data) / (p.numChannels * p.bytesPerSample)
	if err := C.Pa_WriteStream(p.stream, unsafe.Pointer(&data[0]), C.ulong(nbSamples)); err != C.paNoError {
		if err == C.paOutputUnderflowed {
			log.Printf("Underflow\n")
		} else {
			return paError(err)
		}
	}
	return nil
}

func paError(err C.PaError) error {
	return fmt.Errorf("PA Error: %v %v", err, C.GoString(C.Pa_GetErrorText(err)))
}
