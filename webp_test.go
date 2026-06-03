package main

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"testing"
)

func TestEncodeImageWritesWebP(t *testing.T) {
	spec := imageSpec{
		width:  80,
		height: 40,
		color:  color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x78},
		format: formatWebP,
	}

	var buf bytes.Buffer
	if err := encodeImage(&buf, spec); err != nil {
		t.Fatalf("encodeImage returned error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 25 {
		t.Fatalf("encoded data is too short: %d bytes", len(data))
	}
	if string(data[0:4]) != "RIFF" {
		t.Fatalf("RIFF signature = %q, want RIFF", data[0:4])
	}
	if got := int(binary.LittleEndian.Uint32(data[4:8])); got != len(data)-8 {
		t.Fatalf("RIFF size = %d, want %d", got, len(data)-8)
	}
	if string(data[8:12]) != "WEBP" {
		t.Fatalf("WEBP signature = %q, want WEBP", data[8:12])
	}
	if string(data[12:16]) != "VP8L" {
		t.Fatalf("chunk name = %q, want VP8L", data[12:16])
	}

	chunkSize := int(binary.LittleEndian.Uint32(data[16:20]))
	if chunkSize < 5 {
		t.Fatalf("VP8L chunk is too short: %d bytes", chunkSize)
	}
	payload := data[20 : 20+chunkSize]
	if payload[0] != 0x2f {
		t.Fatalf("VP8L signature = %#x, want 0x2f", payload[0])
	}

	header := binary.LittleEndian.Uint32(payload[1:5])
	width := int(header&0x3fff) + 1
	height := int((header>>14)&0x3fff) + 1
	if width != spec.width || height != spec.height {
		t.Fatalf("dimensions = %dx%d, want %dx%d", width, height, spec.width, spec.height)
	}
}
