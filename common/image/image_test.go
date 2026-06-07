package image_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"strconv"
	"strings"
	"testing"

	img "github.com/songquanpeng/one-api/common/image"

	"github.com/stretchr/testify/assert"
	_ "golang.org/x/image/webp"
)

type CountingReader struct {
	reader    io.Reader
	BytesRead int
}

func (r *CountingReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.BytesRead += n
	return n, err
}

var (
	cases = []struct {
		format string
		width  int
		height int
		data   []byte
	}{
		{format: "jpeg", width: 2, height: 3, data: encodeJPEG(2, 3)},
		{format: "png", width: 4, height: 5, data: encodePNG(4, 5)},
		{format: "webp", width: 1, height: 1, data: decodeBase64("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")},
		{format: "gif", width: 6, height: 7, data: encodeGIF(6, 7)},
	}
)

func TestMain(m *testing.M) {
	m.Run()
}

func solidImage(width, height int) *image.RGBA {
	imageValue := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageValue.Set(x, y, color.RGBA{R: uint8(30 + x), G: uint8(60 + y), B: 120, A: 255})
		}
	}
	return imageValue
}

func encodeJPEG(width, height int) []byte {
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, solidImage(width, height), nil); err != nil {
		panic(fmt.Sprintf("encode jpeg fixture: %v", err))
	}
	return buffer.Bytes()
}

func encodePNG(width, height int) []byte {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, solidImage(width, height)); err != nil {
		panic(fmt.Sprintf("encode png fixture: %v", err))
	}
	return buffer.Bytes()
}

func encodeGIF(width, height int) []byte {
	var buffer bytes.Buffer
	if err := gif.Encode(&buffer, solidImage(width, height), nil); err != nil {
		panic(fmt.Sprintf("encode gif fixture: %v", err))
	}
	return buffer.Bytes()
}

func decodeBase64(value string) []byte {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic(fmt.Sprintf("decode base64 fixture: %v", err))
	}
	return data
}

func dataURL(c struct {
	format string
	width  int
	height int
	data   []byte
}) string {
	mimeType := "image/" + c.format
	if c.format == "jpeg" {
		mimeType = "image/jpeg"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(c.data)
}

func TestDecode(t *testing.T) {
	// Bytes read: varies sometimes
	// jpeg: 1063892
	// png: 294462
	// webp: 99529
	// gif: 956153
	// jpeg#01: 32805
	for _, c := range cases {
		t.Run("Decode:"+c.format, func(t *testing.T) {
			reader := &CountingReader{reader: bytes.NewReader(c.data)}
			decodedImage, format, err := image.Decode(reader)
			assert.NoError(t, err)
			size := decodedImage.Bounds().Size()
			assert.Equal(t, c.format, format)
			assert.Equal(t, c.width, size.X)
			assert.Equal(t, c.height, size.Y)
			t.Logf("Bytes read: %d", reader.BytesRead)
		})
	}

	// Bytes read:
	// jpeg: 4096
	// png: 4096
	// webp: 4096
	// gif: 4096
	// jpeg#01: 4096
	for _, c := range cases {
		t.Run("DecodeConfig:"+c.format, func(t *testing.T) {
			reader := &CountingReader{reader: bytes.NewReader(c.data)}
			config, format, err := image.DecodeConfig(reader)
			assert.NoError(t, err)
			assert.Equal(t, c.format, format)
			assert.Equal(t, c.width, config.Width)
			assert.Equal(t, c.height, config.Height)
			t.Logf("Bytes read: %d", reader.BytesRead)
		})
	}
}

func TestBase64(t *testing.T) {
	// Bytes read:
	// jpeg: 1063892
	// png: 294462
	// webp: 99072
	// gif: 953856
	// jpeg#01: 32805
	for _, c := range cases {
		t.Run("Decode:"+c.format, func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString(c.data)
			body := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
			reader := &CountingReader{reader: body}
			decodedImage, format, err := image.Decode(reader)
			assert.NoError(t, err)
			size := decodedImage.Bounds().Size()
			assert.Equal(t, c.format, format)
			assert.Equal(t, c.width, size.X)
			assert.Equal(t, c.height, size.Y)
			t.Logf("Bytes read: %d", reader.BytesRead)
		})
	}

	// Bytes read:
	// jpeg: 1536
	// png: 768
	// webp: 768
	// gif: 1536
	// jpeg#01: 3840
	for _, c := range cases {
		t.Run("DecodeConfig:"+c.format, func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString(c.data)
			body := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
			reader := &CountingReader{reader: body}
			config, format, err := image.DecodeConfig(reader)
			assert.NoError(t, err)
			assert.Equal(t, c.format, format)
			assert.Equal(t, c.width, config.Width)
			assert.Equal(t, c.height, config.Height)
			t.Logf("Bytes read: %d", reader.BytesRead)
		})
	}
}

func TestGetImageSize(t *testing.T) {
	for i, c := range cases {
		t.Run("Decode:"+strconv.Itoa(i), func(t *testing.T) {
			width, height, err := img.GetImageSize(dataURL(c))
			assert.NoError(t, err)
			assert.Equal(t, c.width, width)
			assert.Equal(t, c.height, height)
		})
	}
}

func TestGetImageSizeFromBase64(t *testing.T) {
	for i, c := range cases {
		t.Run("Decode:"+strconv.Itoa(i), func(t *testing.T) {
			encoded := base64.StdEncoding.EncodeToString(c.data)
			width, height, err := img.GetImageSizeFromBase64(encoded)
			assert.NoError(t, err)
			assert.Equal(t, c.width, width)
			assert.Equal(t, c.height, height)
		})
	}
}
