package main

import (
	"image/color"
	"io"
	"net/http/httptest"
	"testing"
)

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want color.NRGBA
	}{
		{name: "rgb short", in: "0fA", want: color.NRGBA{R: 0x00, G: 0xff, B: 0xaa, A: 0xff}},
		{name: "rgb long", in: "123456", want: color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}},
		{name: "argb", in: "80123456", want: color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x80}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseHexColor(tt.in)
			if !ok {
				t.Fatalf("parseHexColor(%q) returned false", tt.in)
			}
			if got != tt.want {
				t.Fatalf("parseHexColor(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseArgsValidatesRanges(t *testing.T) {
	_, err := parseArgs([]string{"--width-min", "20", "--width-max", "10"}, io.Discard)
	if err == nil {
		t.Fatal("parseArgs returned nil error for an invalid width range")
	}
}

func TestParseArgsParsesCacheControl(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", args: nil, want: "max-age=60"},
		{name: "seconds", args: []string{"--cache-control", "120"}, want: "max-age=120"},
		{name: "none", args: []string{"--cache-control", "none"}, want: "no-store"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseArgs(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("parseArgs returned error: %v", err)
			}
			if got := cfg.cache.headerValue(); got != tt.want {
				t.Fatalf("cache header = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseArgsRejectsInvalidCacheControl(t *testing.T) {
	_, err := parseArgs([]string{"--cache-control", "-1"}, io.Discard)
	if err == nil {
		t.Fatal("parseArgs returned nil error for an invalid cache-control")
	}
}

func TestParseArgsParsesAspectRatios(t *testing.T) {
	cfg, err := parseArgs([]string{
		"--width-min", "16",
		"--width-max", "320",
		"--height-min", "9",
		"--height-max", "240",
		"--aspect-ratio", "16:9,4:3",
		"--aspect-ratio", "1920x1080",
	}, io.Discard)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	want := []aspectRatio{
		{width: 16, height: 9},
		{width: 4, height: 3},
		{width: 16, height: 9},
	}
	if len(cfg.ratios) != len(want) {
		t.Fatalf("ratio count = %d, want %d", len(cfg.ratios), len(want))
	}
	for i, wantRatio := range want {
		if cfg.ratios[i] != wantRatio {
			t.Fatalf("ratio[%d] = %#v, want %#v", i, cfg.ratios[i], wantRatio)
		}
	}
}

func TestParseArgsRejectsAspectRatioThatCannotFitRanges(t *testing.T) {
	_, err := parseArgs([]string{
		"--width-min", "100",
		"--width-max", "100",
		"--height-min", "100",
		"--height-max", "100",
		"--aspect-ratio", "16:9",
	}, io.Discard)
	if err == nil {
		t.Fatal("parseArgs returned nil error for an impossible aspect ratio")
	}
}

func TestResolveRequestUsesAspectRatioForRandomDimensions(t *testing.T) {
	cfg := serverConfig{
		port:        defaultPort,
		widthRange:  dimensionRange{min: 160, max: 160},
		heightRange: dimensionRange{min: 90, max: 90},
		ratios:      []aspectRatio{{width: 16, height: 9}},
		cache:       cachePolicy{seconds: defaultCacheSeconds},
	}
	req := httptest.NewRequest("GET", "/", nil)

	got := cfg.resolveRequest(req)

	if got.width != 160 || got.height != 90 {
		t.Fatalf("dimensions = %dx%d, want 160x90", got.width, got.height)
	}
}

func TestResolveRequestKeepsExplicitQueryDimensionsWithAspectRatio(t *testing.T) {
	cfg := serverConfig{
		port:        defaultPort,
		widthRange:  dimensionRange{min: 10, max: 200},
		heightRange: dimensionRange{min: 10, max: 200},
		ratios:      []aspectRatio{{width: 16, height: 9}},
		cache:       cachePolicy{seconds: defaultCacheSeconds},
	}
	req := httptest.NewRequest("GET", "/?width=100&height=100", nil)

	got := cfg.resolveRequest(req)

	if got.width != 100 || got.height != 100 {
		t.Fatalf("dimensions = %dx%d, want explicit query dimensions 100x100", got.width, got.height)
	}
}

func TestResolveRequestClampsQueryAndOverrides(t *testing.T) {
	cfg := serverConfig{
		port:        defaultPort,
		widthRange:  dimensionRange{min: 10, max: 20},
		heightRange: dimensionRange{min: 30, max: 40},
		color:       color.NRGBA{R: 1, G: 2, B: 3, A: 0xff},
		hasColor:    true,
		format:      formatPNG,
		hasFormat:   true,
		cache:       cachePolicy{seconds: defaultCacheSeconds},
	}
	req := httptest.NewRequest("GET", "/ignored/path?width=999&height=1&color=80123456&format=webp", nil)

	got := cfg.resolveRequest(req)

	if got.width != 20 {
		t.Fatalf("width = %d, want 20", got.width)
	}
	if got.height != 30 {
		t.Fatalf("height = %d, want 30", got.height)
	}
	wantColor := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x80}
	if got.color != wantColor {
		t.Fatalf("color = %#v, want %#v", got.color, wantColor)
	}
	if got.format != formatWebP {
		t.Fatalf("format = %q, want %q", got.format, formatWebP)
	}
}

func TestResolveRequestIgnoresInvalidQuery(t *testing.T) {
	wantColor := color.NRGBA{R: 10, G: 20, B: 30, A: 0xff}
	cfg := serverConfig{
		port:        defaultPort,
		widthRange:  dimensionRange{min: 12, max: 12},
		heightRange: dimensionRange{min: 34, max: 34},
		color:       wantColor,
		hasColor:    true,
		format:      formatJPG,
		hasFormat:   true,
		cache:       cachePolicy{seconds: defaultCacheSeconds},
	}
	req := httptest.NewRequest("GET", "/?width=0&height=bad&color=not-hex&format=gif", nil)

	got := cfg.resolveRequest(req)

	if got.width != 12 || got.height != 34 {
		t.Fatalf("dimensions = %dx%d, want 12x34", got.width, got.height)
	}
	if got.color != wantColor {
		t.Fatalf("color = %#v, want %#v", got.color, wantColor)
	}
	if got.format != formatJPG {
		t.Fatalf("format = %q, want %q", got.format, formatJPG)
	}
}
