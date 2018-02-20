package jukybox

/*
#cgo pkg-config: libcec
#cgo CXXFLAGS: -std=c++11
#include "remote_cec.h"
*/
import "C"

import (
	"log"
	"sync"
)

type CECRemote struct {
	buttonEvents     chan<- Button
	remote           *C.CECRemote
	handleCommandCB  int
	handleKeyPressCB int
}

func CreateCECRemote(buttonEvents chan<- Button) *CECRemote {
	result := CECRemote{}
	result.buttonEvents = buttonEvents
	result.handleKeyPressCB = register(func(c C.int) { result.handleKeyPress(int(c)) })
	result.handleCommandCB = register(func(c C.int) { result.handleCommand(int(c)) })
	result.remote = C.newCECRemote(C.int(result.handleKeyPressCB), C.int(result.handleCommandCB))
	return &result
}

func (r *CECRemote) Destroy() {
	C.deleteCECRemote(r.remote)
	unregister(r.handleKeyPressCB)
	unregister(r.handleCommandCB)
}

func (r *CECRemote) handleKeyPress(code int) {
	log.Printf("CEC: KeyPress 0x%x", code)
	switch code {
	case 0x1:
		r.buttonEvents <- PreviousAlbumButton
	case 0x2:
		r.buttonEvents <- NextAlbumButton
	case 0x3:
		r.buttonEvents <- PreviousTrackButton
	case 0x4:
		r.buttonEvents <- NextTrackButton
		// case 0x49:
		// 	r.buttonEvents <- FastForwardButton
		// case 0x48:
		// 	r.buttonEvents <- RewindButton

	}
}

func (r *CECRemote) handleCommand(code int) {
	log.Printf("CEC: Command 0x%x", code)
	switch code {
	case 0x41:
		r.buttonEvents <- PlayPauseButton
	}
}

////////////////////////////////////////////////////////////////////////////////
// Callback handling
////////////////////////////////////////////////////////////////////////////////

//export go_callback_int
func go_callback_int(cb C.int, a1 C.int) {
	fn := lookup(int(cb))
	fn(a1)
}

var mu sync.Mutex
var index int
var fns = make(map[int]func(C.int))

func register(fn func(C.int)) int {
	mu.Lock()
	defer mu.Unlock()
	index++
	for fns[index] != nil {
		index++
	}
	fns[index] = fn
	return index
}

func lookup(i int) func(C.int) {
	mu.Lock()
	defer mu.Unlock()
	return fns[i]
}

func unregister(i int) {
	mu.Lock()
	defer mu.Unlock()
	delete(fns, i)
}
