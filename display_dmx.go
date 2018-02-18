// +build arm

package jukybox

/*
#cgo pkg-config: bcm_host
#include <bcm_host.h>
*/
import "C"

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"unsafe"
)

func must(ret C.int) {
	if ret != 0 {
		log.Fatalf("DMX Error")
	}
}

type DMXDisplay struct {
	display  C.DISPMANX_DISPLAY_HANDLE_T
	resource C.DISPMANX_RESOURCE_HANDLE_T
	element  C.DISPMANX_ELEMENT_HANDLE_T
	image    []uint16
	drawer   *DisplayDrawer
	events   chan DisplayInfo
}

func CreateDMXDisplay() *DMXDisplay {
	C.bcm_host_init()
	d := DMXDisplay{
		drawer: CreateDisplayDrawer(),
		events: make(chan DisplayInfo),
	}
	d.display = C.vc_dispmanx_display_open(0)

	var info C.DISPMANX_MODEINFO_T
	must(C.vc_dispmanx_display_get_info(d.display, &info))

	var imagePtr C.uint32_t
	d.resource = C.vc_dispmanx_resource_create(C.VC_IMAGE_RGB565, DISPLAY_WIDTH, DISPLAY_HEIGHT, &imagePtr)
	if d.resource == 0 {
		log.Fatalf("DMX Error")
	}

	d.image = make([]uint16, DISPLAY_WIDTH*2*DISPLAY_HEIGHT)
	var sr, dr C.VC_RECT_T
	C.vc_dispmanx_rect_set(&dr, 0, 0, DISPLAY_WIDTH, DISPLAY_HEIGHT)
	must(C.vc_dispmanx_resource_write_data(d.resource, C.VC_IMAGE_RGB565, 2*DISPLAY_WIDTH, unsafe.Pointer(&d.image[0]), &dr))

	update := C.vc_dispmanx_update_start(10)
	C.vc_dispmanx_rect_set(&sr, 0, 0, DISPLAY_WIDTH<<16, DISPLAY_HEIGHT<<16)
	C.vc_dispmanx_rect_set(&dr, 0, 0, C.uint32_t(info.width), C.uint32_t(info.height))
	d.element = C.vc_dispmanx_element_add(update, d.display, 1, &dr, d.resource, &sr, C.DISPMANX_PROTECTION_NONE, nil, nil, C.VC_IMAGE_ROT0)
	must(C.vc_dispmanx_update_submit_sync(update))

	return &d
}

func (d *DMXDisplay) Run() {
	stopped := false
	for !stopped {
		select {
		case info, ok := <-d.events:
			if !ok {
				stopped = true
			} else {
				d.drawer.Draw(d, info)
			}
		}
	}

	update := C.vc_dispmanx_update_start(10)
	must(C.vc_dispmanx_element_remove(update, d.element))
	must(C.vc_dispmanx_update_submit_sync(update))
	must(C.vc_dispmanx_resource_delete(d.resource))
	must(C.vc_dispmanx_display_close(d.display))
}

func (d *DMXDisplay) Stop() {
	close(d.events)
}

func (d *DMXDisplay) Flush() {
	var rect C.VC_RECT_T
	C.vc_dispmanx_rect_set(&rect, 0, 0, DISPLAY_WIDTH, DISPLAY_HEIGHT)
	must(C.vc_dispmanx_resource_write_data(d.resource, C.VC_IMAGE_RGB565, 2*DISPLAY_WIDTH, unsafe.Pointer(&d.image[0]), &rect))
}

type LCDImage struct {
	image []uint16
}

func (i LCDImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (i LCDImage) Set(x, y int, c color.Color) {
	r, _, _, _ := c.RGBA()
	if r == 0 {
		i.image[y*DISPLAY_WIDTH+x] = 0
	} else {
		i.image[y*DISPLAY_WIDTH+x] = 0xFFFF
	}
}

func (i LCDImage) Bounds() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{DISPLAY_WIDTH, DISPLAY_HEIGHT},
	}
}

func (i LCDImage) At(int, int) color.Color {
	// Not implemented
	return color.White
}

func (d *DMXDisplay) Image() draw.Image {
	return LCDImage{d.image}
}

func (d *DMXDisplay) Draw(info DisplayInfo) {
	d.events <- info
}
