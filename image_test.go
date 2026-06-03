package main

import (
	"bytes"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImageHandlerIgnoresPathAndReturnsPNG(t *testing.T) {
	wantColor := color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	width := 80
	height := 40
	handler := imageHandler{
		cfg: serverConfig{
			port:        defaultPort,
			widthRange:  dimensionRange{min: width, max: width},
			heightRange: dimensionRange{min: height, max: height},
			color:       wantColor,
			hasColor:    true,
			format:      formatPNG,
			hasFormat:   true,
			cache:       cachePolicy{seconds: defaultCacheSeconds},
			label:       true,
			quality:     defaultQuality,
		},
	}

	req := httptest.NewRequest("GET", "/any/path/is/ignored", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "max-age=60" {
		t.Fatalf("Cache-Control = %q, want max-age=60", got)
	}

	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("png.Decode returned error: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Fatalf("bounds = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
	}

	gotBackground := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if gotBackground != wantColor {
		t.Fatalf("background pixel = %#v, want %#v", gotBackground, wantColor)
	}

	label := newLabelImage(width, height, wantColor, true)
	x, y, ok := firstLabelPixel(label)
	if !ok {
		t.Fatal("label image did not produce any label pixels")
	}
	gotText := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if gotText != label.fg {
		t.Fatalf("label pixel = %#v, want %#v", gotText, label.fg)
	}
}

func TestEncodePNGCanDisableLabel(t *testing.T) {
	bg := color.NRGBA{R: 0xee, G: 0xdd, B: 0xcc, A: 0xff}
	spec := imageSpec{
		width:   80,
		height:  40,
		color:   bg,
		format:  formatPNG,
		label:   false,
		quality: defaultQuality,
	}

	var buf bytes.Buffer
	if err := encodeImage(&buf, spec); err != nil {
		t.Fatalf("encodeImage returned error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("png.Decode returned error: %v", err)
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got != bg {
				t.Fatalf("pixel at %d,%d = %#v, want %#v", x, y, got, bg)
			}
		}
	}
}

func firstLabelPixel(img labelImage) (int, int, bool) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.isLabelPixel(x, y) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}
