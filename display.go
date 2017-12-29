//go:generate go-bindata -pkg jukybox -o fonts.go fonts/...

package jukybox

import (
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"image"
	"image/draw"
	"log"
	"time"
)

const (
	DISPLAY_WIDTH  = 128
	DISPLAY_HEIGHT = 64
)

const (
	REGULAR_FONT_SIZE = 13
	BOLD_FONT_SIZE    = 14
	ITALIC_FONT_SIZE  = 13
	SYMBOLS_FONT_SIZE = 16
)

const (
	POSITION_X      = 16
	POSITION_Y      = DISPLAY_HEIGHT - 2
	POSITION_MARGIN = 1
	POSITION_HEIGHT = 4
)

type DisplayInfo struct {
	title    string
	artist   string
	position time.Duration
	duration time.Duration

	chapterTitle    string
	chapterIndex    int
	chapterPosition time.Duration
	chapterDuration time.Duration

	stateIcon string
}

type DisplayDrawer struct {
	regularCtx *freetype.Context
	boldCtx    *freetype.Context
	italicCtx  *freetype.Context
	symbolsCtx *freetype.Context
}

func createFontContext(fontFile string, size float64) *freetype.Context {
	b, err := Asset(fontFile)
	if err != nil {
		log.Fatal(err)
	}
	font, err := truetype.Parse(b)
	if err != nil {
		log.Fatal(err)
	}
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(size)
	c.SetSrc(image.White)
	return c
}

func CreateDisplayDrawer() *DisplayDrawer {
	return &DisplayDrawer{
		regularCtx: createFontContext("fonts/NotoSansDisplay-Condensed.ttf", REGULAR_FONT_SIZE),
		boldCtx:    createFontContext("fonts/NotoSansDisplay-CondensedBold.ttf", BOLD_FONT_SIZE),
		italicCtx:  createFontContext("fonts/NotoSansDisplay-CondensedItalic.ttf", ITALIC_FONT_SIZE),
		symbolsCtx: createFontContext("fonts/NotoSansSymbols2-Regular.ttf", SYMBOLS_FONT_SIZE),
	}
}

func (d *DisplayDrawer) Draw(display *Display, info DisplayInfo) {
	s := display.Image()
	draw.Draw(s, s.Bounds(), image.Black, image.ZP, draw.Src)
	d.boldCtx.SetClip(s.Bounds())
	d.boldCtx.SetDst(s)
	line1Offset := 2 + d.boldCtx.PointToFixed(BOLD_FONT_SIZE)>>6
	pt := freetype.Pt(0, int(line1Offset))
	if _, err := d.boldCtx.DrawString(info.title, pt); err != nil {
		log.Fatal(err)
	}

	d.italicCtx.SetClip(s.Bounds())
	d.italicCtx.SetDst(s)
	line2Offset := line1Offset + (d.boldCtx.PointToFixed(ITALIC_FONT_SIZE) >> 6) + 1
	pt = freetype.Pt(0, int(line2Offset))
	if _, err := d.italicCtx.DrawString(info.artist, pt); err != nil {
		log.Fatal(err)
	}

	d.regularCtx.SetClip(s.Bounds())
	d.regularCtx.SetDst(s)
	line3Offset := line2Offset + (d.boldCtx.PointToFixed(REGULAR_FONT_SIZE) >> 6) + 1
	pt = freetype.Pt(0, int(line3Offset))
	if _, err := d.regularCtx.DrawString(info.chapterTitle, pt); err != nil {
		log.Fatal(err)
	}

	d.symbolsCtx.SetClip(s.Bounds())
	d.symbolsCtx.SetDst(s)
	line4Offset := DISPLAY_HEIGHT - 3
	pt = freetype.Pt(0, int(line4Offset))
	if _, err := d.symbolsCtx.DrawString(info.stateIcon, pt); err != nil {
		log.Fatal(err)
	}

	if info.duration > 0 {
		draw.Draw(s, image.Rectangle{
			Min: image.Point{
				X: POSITION_X,
				Y: POSITION_Y - (2 * (POSITION_HEIGHT + 2*POSITION_MARGIN)) + POSITION_MARGIN,
			},
			Max: image.Point{
				X: POSITION_X + int((DISPLAY_WIDTH-POSITION_X)*(float64(info.position)/float64(info.duration))),
				Y: POSITION_Y - (POSITION_HEIGHT + 2*POSITION_MARGIN) - POSITION_MARGIN,
			},
		}, image.White, image.ZP, draw.Src)
	}
	if info.chapterDuration > 0 {
		draw.Draw(s, image.Rectangle{
			Min: image.Point{
				X: POSITION_X,
				Y: POSITION_Y - (POSITION_HEIGHT + 2*POSITION_MARGIN) + POSITION_MARGIN,
			},
			Max: image.Point{
				X: POSITION_X + int((DISPLAY_WIDTH-POSITION_X)*(float64(info.chapterPosition)/float64(info.chapterDuration))),
				Y: POSITION_Y - POSITION_MARGIN,
			},
		}, image.White, image.ZP, draw.Src)
	}

	display.Flush()
}
