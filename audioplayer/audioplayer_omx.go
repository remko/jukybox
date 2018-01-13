// +build arm

package audioplayer

/*
#cgo pkg-config: bcm_host
#cgo CPPFLAGS: -DOMX_SKIP64BIT -I/opt/vc/src/hello_pi/libs/ilclient
#cgo LDFLAGS: -L/opt/vc/lib/ -lvcos -lvchiq_arm -lpthread -lopenmaxil -L/opt/vc/src/hello_pi/libs/ilclient -lilclient

#include <bcm_host.h>

#include "audioplayer_omx.h"

*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

var omxInitializeOnce sync.Once

type OMXAudioPlayer struct {
	client *C.OMXClient
}

func Create() (*OMXAudioPlayer, error) {
	omxInitializeOnce.Do(func() {
		C.bcm_host_init()
	})

	client := C.OMXClient_Create()
	if client == nil {
		return nil, fmt.Errorf("createOMXClient")
	}
	return &OMXAudioPlayer{
		client: client,
	}, nil
}

func (p *OMXAudioPlayer) Start(numChannels int, bytesPerSample int, sampleRate int, isFloatPlanar bool, encoding string) error {
	cIsFloatPlanar := 0
	if isFloatPlanar {
		cIsFloatPlanar = 1
	}
	var cEncoding C.OMXClientEncoding
	switch encoding {
	case "dts":
		cEncoding = C.OMXClientEncoding_DTS
	case "ac3", "eac3":
		cEncoding = C.OMXClientEncoding_DDP
	default:
		cEncoding = C.OMXClientEncoding_PCM
	}
	if ret := C.OMXClient_Start(p.client, C.int(numChannels), C.int(bytesPerSample<<3), C.int(sampleRate), C.int(cIsFloatPlanar), cEncoding); ret != 0 {
		return fmt.Errorf("error start")
	}
	return nil
}

func (p *OMXAudioPlayer) Stop() {
	C.OMXClient_Stop(p.client)
}

func (p *OMXAudioPlayer) NumOutputChannels() int {
	return 8
}

func (p *OMXAudioPlayer) Write(data []byte) error {
	if err := C.OMXClient_Write(p.client, (*C.char)(unsafe.Pointer(&data[0])), C.int(len(data))); err != 0 {
		return fmt.Errorf("error writing")
	}
	return nil
}

func omxError(msg string, err C.OMX_ERRORTYPE) error {
	return fmt.Errorf("%s: %d", msg, err)
}

func IsPassthroughSupported(codec string, codecProfile string, samplerate int) bool {
	switch codec {
	case "dts":
		return samplerate != 44100
	case "eac3", "ac3":
		return true
	default:
		return false
	}
}
