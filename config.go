package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultPort         = 8080
	defaultMinDimension = 1
	defaultMaxDimension = 1024
	defaultCacheSeconds = 60
	maxDimension        = 16384
)

type imageFormat string

const (
	formatJPG  imageFormat = "jpg"
	formatPNG  imageFormat = "png"
	formatWebP imageFormat = "webp"
)

var supportedFormats = []imageFormat{formatJPG, formatPNG, formatWebP}

type dimensionRange struct {
	min int
	max int
}

func (r dimensionRange) random() int {
	if r.min == r.max {
		return r.min
	}
	return r.min + rand.IntN(r.max-r.min+1)
}

func (r dimensionRange) clamp(v int) int {
	if v < r.min {
		return r.min
	}
	if v > r.max {
		return r.max
	}
	return v
}

type serverConfig struct {
	port        int
	widthRange  dimensionRange
	heightRange dimensionRange
	ratios      []aspectRatio
	color       color.NRGBA
	hasColor    bool
	format      imageFormat
	hasFormat   bool
	cache       cachePolicy
}

type imageSpec struct {
	width  int
	height int
	color  color.NRGBA
	format imageFormat
}

type cachePolicy struct {
	noStore bool
	seconds int
}

type aspectRatio struct {
	width  int
	height int
}

type aspectRatioFlag []aspectRatio

func (f *aspectRatioFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}

	parts := make([]string, 0, len(*f))
	for _, ratio := range *f {
		parts = append(parts, strconv.Itoa(ratio.width)+":"+strconv.Itoa(ratio.height))
	}
	return strings.Join(parts, ",")
}

func (f *aspectRatioFlag) Set(s string) error {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		ratio, err := parseAspectRatio(part)
		if err != nil {
			return err
		}
		*f = append(*f, ratio)
	}
	return nil
}

func (p cachePolicy) headerValue() string {
	if p.noStore {
		return "no-store"
	}
	return "max-age=" + strconv.Itoa(p.seconds)
}

func parseArgs(args []string, output io.Writer) (serverConfig, error) {
	var (
		port         int
		widthMin     int
		widthMax     int
		heightMin    int
		heightMax    int
		colorHex     string
		format       string
		cacheControl string
		ratios       aspectRatioFlag
	)

	fs := flag.NewFlagSet("imagemock", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.IntVar(&port, "port", defaultPort, "listening port")
	fs.IntVar(&widthMin, "width-min", 0, "minimum image width")
	fs.IntVar(&widthMax, "width-max", 0, "maximum image width")
	fs.IntVar(&heightMin, "height-min", 0, "minimum image height")
	fs.IntVar(&heightMax, "height-max", 0, "maximum image height")
	fs.StringVar(&colorHex, "color", "", "fixed color in RGB, RRGGBB, or AARRGGBB hex")
	fs.StringVar(&format, "format", "", "fixed format: jpg, png, or webp")
	fs.StringVar(&cacheControl, "cache-control", strconv.Itoa(defaultCacheSeconds), "browser cache duration in seconds, or none")
	fs.Var(&ratios, "aspect-ratio", "allowed aspect ratio such as 16:9; repeat or separate with commas")

	if err := fs.Parse(args); err != nil {
		return serverConfig{}, err
	}
	if fs.NArg() != 0 {
		return serverConfig{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	cfg := serverConfig{
		port:        port,
		widthRange:  normalizeRange(widthMin, widthMax),
		heightRange: normalizeRange(heightMin, heightMax),
		ratios:      []aspectRatio(ratios),
	}
	if err := cfg.validate(); err != nil {
		return serverConfig{}, err
	}

	if colorHex != "" {
		c, ok := parseHexColor(colorHex)
		if !ok {
			return serverConfig{}, fmt.Errorf("invalid color %q", colorHex)
		}
		cfg.color = c
		cfg.hasColor = true
	}

	if format != "" {
		f, ok := parseFormat(format)
		if !ok {
			return serverConfig{}, fmt.Errorf("invalid format %q", format)
		}
		cfg.format = f
		cfg.hasFormat = true
	}

	cache, err := parseCachePolicy(cacheControl)
	if err != nil {
		return serverConfig{}, err
	}
	cfg.cache = cache

	return cfg, nil
}

func normalizeRange(minValue, maxValue int) dimensionRange {
	if minValue == 0 {
		minValue = defaultMinDimension
	}
	if maxValue == 0 {
		maxValue = defaultMaxDimension
	}
	return dimensionRange{min: minValue, max: maxValue}
}

func (cfg serverConfig) validate() error {
	if cfg.port < 1 || cfg.port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535: %d", cfg.port)
	}
	if err := validateRange("width", cfg.widthRange); err != nil {
		return err
	}
	if err := validateRange("height", cfg.heightRange); err != nil {
		return err
	}
	for _, ratio := range cfg.ratios {
		if _, _, ok := ratio.randomDimensions(cfg.widthRange, cfg.heightRange); !ok {
			return fmt.Errorf("aspect ratio %d:%d cannot fit in width range %d..%d and height range %d..%d", ratio.width, ratio.height, cfg.widthRange.min, cfg.widthRange.max, cfg.heightRange.min, cfg.heightRange.max)
		}
	}
	return nil
}

func validateRange(name string, r dimensionRange) error {
	if r.min < 1 {
		return fmt.Errorf("%s-min must be at least 1: %d", name, r.min)
	}
	if r.max > maxDimension {
		return fmt.Errorf("%s-max must be %d or less: %d", name, maxDimension, r.max)
	}
	if r.min > r.max {
		return fmt.Errorf("%s-min must be less than or equal to %s-max: %d > %d", name, name, r.min, r.max)
	}
	return nil
}

func (cfg serverConfig) resolveRequest(r *http.Request) imageSpec {
	q := r.URL.Query()
	width, height := cfg.randomDimensions()

	spec := imageSpec{
		width:  width,
		height: height,
		color:  randomColor(),
		format: randomFormat(),
	}
	if cfg.hasColor {
		spec.color = cfg.color
	}
	if cfg.hasFormat {
		spec.format = cfg.format
	}

	if width, ok := parsePositiveInt(q.Get("width")); ok {
		spec.width = cfg.widthRange.clamp(width)
	}
	if height, ok := parsePositiveInt(q.Get("height")); ok {
		spec.height = cfg.heightRange.clamp(height)
	}
	if c, ok := parseHexColor(q.Get("color")); ok {
		spec.color = c
	}
	if f, ok := parseFormat(q.Get("format")); ok {
		spec.format = f
	}

	return spec
}

func (cfg serverConfig) randomDimensions() (int, int) {
	if len(cfg.ratios) == 0 {
		return cfg.widthRange.random(), cfg.heightRange.random()
	}

	ratio := cfg.ratios[rand.IntN(len(cfg.ratios))]
	width, height, ok := ratio.randomDimensions(cfg.widthRange, cfg.heightRange)
	if !ok {
		return cfg.widthRange.random(), cfg.heightRange.random()
	}
	return width, height
}

func parsePositiveInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}

func parseCachePolicy(s string) (cachePolicy, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "none", "no-cache", "off":
		return cachePolicy{noStore: true}, nil
	}
	seconds, err := strconv.Atoi(s)
	if err != nil || seconds < 0 {
		return cachePolicy{}, fmt.Errorf("cache-control must be a non-negative second value or none: %q", s)
	}
	return cachePolicy{seconds: seconds}, nil
}

func parseAspectRatio(s string) (aspectRatio, error) {
	separator := ":"
	if strings.Contains(s, "x") {
		separator = "x"
	}

	parts := strings.Split(s, separator)
	if len(parts) != 2 {
		return aspectRatio{}, fmt.Errorf("aspect-ratio must be formatted as width:height: %q", s)
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width < 1 {
		return aspectRatio{}, fmt.Errorf("aspect-ratio width must be positive: %q", s)
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height < 1 {
		return aspectRatio{}, fmt.Errorf("aspect-ratio height must be positive: %q", s)
	}
	if width > maxDimension || height > maxDimension {
		return aspectRatio{}, fmt.Errorf("aspect-ratio values must be %d or less: %q", maxDimension, s)
	}

	divisor := gcd(width, height)
	return aspectRatio{width: width / divisor, height: height / divisor}, nil
}

func (ratio aspectRatio) randomDimensions(widthRange, heightRange dimensionRange) (int, int, bool) {
	minScale := max(ceilDiv(widthRange.min, ratio.width), ceilDiv(heightRange.min, ratio.height))
	maxScale := min(widthRange.max/ratio.width, heightRange.max/ratio.height)
	if minScale > maxScale {
		return 0, 0, false
	}

	scale := minScale
	if minScale != maxScale {
		scale += rand.IntN(maxScale - minScale + 1)
	}
	return ratio.width * scale, ratio.height * scale, true
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func parseHexColor(s string) (color.NRGBA, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		r, ok := parseHexNibble(s[0])
		if !ok {
			return color.NRGBA{}, false
		}
		g, ok := parseHexNibble(s[1])
		if !ok {
			return color.NRGBA{}, false
		}
		b, ok := parseHexNibble(s[2])
		if !ok {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: r<<4 | r, G: g<<4 | g, B: b<<4 | b, A: 0xff}, true
	case 6:
		r, ok := parseHexByte(s[0:2])
		if !ok {
			return color.NRGBA{}, false
		}
		g, ok := parseHexByte(s[2:4])
		if !ok {
			return color.NRGBA{}, false
		}
		b, ok := parseHexByte(s[4:6])
		if !ok {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: r, G: g, B: b, A: 0xff}, true
	case 8:
		a, ok := parseHexByte(s[0:2])
		if !ok {
			return color.NRGBA{}, false
		}
		r, ok := parseHexByte(s[2:4])
		if !ok {
			return color.NRGBA{}, false
		}
		g, ok := parseHexByte(s[4:6])
		if !ok {
			return color.NRGBA{}, false
		}
		b, ok := parseHexByte(s[6:8])
		if !ok {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: r, G: g, B: b, A: a}, true
	default:
		return color.NRGBA{}, false
	}
}

func parseHexByte(s string) (uint8, bool) {
	if len(s) != 2 {
		return 0, false
	}
	hi, ok := parseHexNibble(s[0])
	if !ok {
		return 0, false
	}
	lo, ok := parseHexNibble(s[1])
	if !ok {
		return 0, false
	}
	return hi<<4 | lo, true
}

func parseHexNibble(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func parseFormat(s string) (imageFormat, bool) {
	switch imageFormat(strings.ToLower(strings.TrimSpace(s))) {
	case formatJPG:
		return formatJPG, true
	case formatPNG:
		return formatPNG, true
	case formatWebP:
		return formatWebP, true
	default:
		return "", false
	}
}

func randomColor() color.NRGBA {
	return color.NRGBA{
		R: uint8(rand.IntN(256)),
		G: uint8(rand.IntN(256)),
		B: uint8(rand.IntN(256)),
		A: 0xff,
	}
}

func randomFormat() imageFormat {
	return supportedFormats[rand.IntN(len(supportedFormats))]
}

func contentType(format imageFormat) (string, error) {
	switch format {
	case formatJPG:
		return "image/jpeg", nil
	case formatPNG:
		return "image/png", nil
	case formatWebP:
		return "image/webp", nil
	default:
		return "", errors.New("unsupported image format")
	}
}
