package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image/jpeg"
	"io"

	webp "github.com/mayahiro/go-webp"
)

func encodeImage(w io.Writer, spec imageSpec) error {
	switch spec.format {
	case formatJPG:
		c := spec.color
		c.A = 0xff
		img := newLabelImage(spec.width, spec.height, c)
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 80})
	case formatPNG:
		img := newLabelImage(spec.width, spec.height, spec.color)
		return encodePNG(w, img)
	case formatWebP:
		img := newLabelImage(spec.width, spec.height, spec.color)
		return webp.Encode(w, img, nil)
	default:
		return errUnsupportedFormat(spec.format)
	}
}

func errUnsupportedFormat(format imageFormat) error {
	_, err := contentType(format)
	return err
}

func encodePNG(w io.Writer, img labelImage) error {
	if _, err := w.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}); err != nil {
		return err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8
	ihdr[9] = 6
	if err := writePNGChunk(w, "IHDR", ihdr[:]); err != nil {
		return err
	}

	var compressed bytes.Buffer
	zw, err := zlib.NewWriterLevel(&compressed, zlib.BestSpeed)
	if err != nil {
		return err
	}

	row := make([]byte, 1+width*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.NRGBAAt(x, y)
			i := 1 + x*4
			row[i] = c.R
			row[i+1] = c.G
			row[i+2] = c.B
			row[i+3] = c.A
		}
		if _, err := zw.Write(row); err != nil {
			_ = zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}

	if err := writePNGChunk(w, "IDAT", compressed.Bytes()); err != nil {
		return err
	}
	return writePNGChunk(w, "IEND", nil)
}

func writePNGChunk(w io.Writer, name string, data []byte) error {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	if _, err := w.Write(length[:]); err != nil {
		return err
	}

	crc := crc32.NewIEEE()
	if _, err := io.WriteString(w, name); err != nil {
		return err
	}
	_, _ = io.WriteString(crc, name)
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, _ = crc.Write(data)

	var checksum [4]byte
	binary.BigEndian.PutUint32(checksum[:], crc.Sum32())
	_, err := w.Write(checksum[:])
	return err
}
