package jukybox

import (
	"image"
	"image/color"
	"image/draw"
)

type Display struct {
}

func CreateDisplay(buttonChannel chan<- Button) *Display {
	return &Display{}
}

func (d *Display) Run() {
}

func (d *Display) Stop() {
}

func (d *Display) Flush() {
}

type LCDImage struct {
}

func (i LCDImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (i LCDImage) Set(x, y int, c color.Color) {
}

func (i LCDImage) Bounds() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{0, 0},
		Max: image.Point{DISPLAY_WIDTH, DISPLAY_HEIGHT},
	}
}

func (i LCDImage) At(int, int) color.Color {
	return color.White
}

func (d *Display) Image() draw.Image {
	return LCDImage{}
}

func (d *Display) Draw(info DisplayInfo) {
}
