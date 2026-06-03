package main

import (
	"fmt"
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	labelMaxWidthRatio  = 0.9
	labelMaxHeightRatio = 0.3
	labelBaseFontSize   = 100.0
	labelMinFontSize    = 4.0
	labelMaxFontSize    = 4096.0
	labelMaskThreshold  = 128
)

var labelFont = mustParseLabelFont()

type labelImage struct {
	rect      image.Rectangle
	bg        color.NRGBA
	fg        color.NRGBA
	label     string
	labelLeft int
	labelTop  int
	mask      *image.Alpha
}

func newLabelImage(width, height int, bg color.NRGBA) labelImage {
	label := fmt.Sprintf("%d x %d", width, height)
	mask := renderLabelMask(width, height, label)

	return labelImage{
		rect:      image.Rect(0, 0, width, height),
		bg:        bg,
		fg:        readableTextColor(bg),
		label:     label,
		labelLeft: (width - mask.Bounds().Dx()) / 2,
		labelTop:  (height - mask.Bounds().Dy()) / 2,
		mask:      mask,
	}
}

func (img labelImage) ColorModel() color.Model {
	return color.NRGBAModel
}

func (img labelImage) Bounds() image.Rectangle {
	return img.rect
}

func (img labelImage) At(x, y int) color.Color {
	return img.NRGBAAt(x, y)
}

func (img labelImage) NRGBAAt(x, y int) color.NRGBA {
	if img.isLabelPixel(x, y) {
		return img.fg
	}
	return img.bg
}

func (img labelImage) Opaque() bool {
	return img.bg.A == 0xff && img.fg.A == 0xff
}

func (img labelImage) isLabelPixel(x, y int) bool {
	if img.mask == nil {
		return false
	}

	maskX := x - img.labelLeft
	maskY := y - img.labelTop
	if maskX < 0 || maskY < 0 || maskX >= img.mask.Bounds().Dx() || maskY >= img.mask.Bounds().Dy() {
		return false
	}

	return img.mask.AlphaAt(maskX, maskY).A >= labelMaskThreshold
}

func renderLabelMask(width, height int, label string) *image.Alpha {
	fontSize := chooseLabelFontSize(width, height, label)
	mask, err := drawLabelMask(label, fontSize)
	if err != nil {
		return image.NewAlpha(image.Rect(0, 0, 1, 1))
	}

	maxWidth := max(1, int(float64(width)*labelMaxWidthRatio))
	maxHeight := max(1, int(float64(height)*labelMaxHeightRatio))
	if height < 40 {
		maxHeight = max(1, height)
	}

	for (mask.Bounds().Dx() > maxWidth || mask.Bounds().Dy() > maxHeight) && fontSize > labelMinFontSize {
		widthRatio := float64(maxWidth) / float64(max(1, mask.Bounds().Dx()))
		heightRatio := float64(maxHeight) / float64(max(1, mask.Bounds().Dy()))
		fontSize *= min(widthRatio, heightRatio) * 0.95
		if fontSize < labelMinFontSize {
			fontSize = labelMinFontSize
		}

		nextMask, err := drawLabelMask(label, fontSize)
		if err != nil {
			break
		}
		mask = nextMask
	}

	return mask
}

func chooseLabelFontSize(width, height int, label string) float64 {
	baseMask, err := drawLabelMask(label, labelBaseFontSize)
	if err != nil {
		return labelMinFontSize
	}

	maxWidth := max(1, int(float64(width)*labelMaxWidthRatio))
	maxHeight := max(1, int(float64(height)*labelMaxHeightRatio))
	if height < 40 {
		maxHeight = max(1, height)
	}

	widthRatio := float64(maxWidth) / float64(max(1, baseMask.Bounds().Dx()))
	heightRatio := float64(maxHeight) / float64(max(1, baseMask.Bounds().Dy()))
	size := labelBaseFontSize * min(widthRatio, heightRatio)
	if size < labelMinFontSize {
		return labelMinFontSize
	}
	if size > labelMaxFontSize {
		return labelMaxFontSize
	}
	return size
}

func drawLabelMask(label string, fontSize float64) (*image.Alpha, error) {
	face, err := opentype.NewFace(labelFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	drawer := &font.Drawer{Face: face}
	bounds, _ := drawer.BoundString(label)
	rect := fixedRectToImageRect(bounds)
	if rect.Empty() {
		return image.NewAlpha(image.Rect(0, 0, 1, 1)), nil
	}

	mask := image.NewAlpha(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	drawer.Dst = mask
	drawer.Src = image.NewUniform(color.Alpha{A: 0xff})
	drawer.Dot = fixed.P(-rect.Min.X, -rect.Min.Y)
	drawer.DrawString(label)
	return mask, nil
}

func fixedRectToImageRect(r fixed.Rectangle26_6) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{
			X: floorFixed(r.Min.X),
			Y: floorFixed(r.Min.Y),
		},
		Max: image.Point{
			X: ceilFixed(r.Max.X),
			Y: ceilFixed(r.Max.Y),
		},
	}
}

func floorFixed(v fixed.Int26_6) int {
	return int(v >> 6)
}

func ceilFixed(v fixed.Int26_6) int {
	return int((v + 63) >> 6)
}

func readableTextColor(bg color.NRGBA) color.NRGBA {
	luminance := int(bg.R)*299 + int(bg.G)*587 + int(bg.B)*114
	if luminance >= 128000 {
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	}
	return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
}

func mustParseLabelFont() *opentype.Font {
	font, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
	return font
}
