package imgproc

import (
	"github.com/nfnt/resize"
	"google.golang.org/cloud/storage"

	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

const MAX_SIZE = 100
const JPEG = "image/jpeg"
const JPG = "image/jpg"
const PNG = "image/png"
const GIF = "image/gif"

func makeThumbnail(filetype string, r io.Reader, w *storage.Writer) error {
	img, err := decodeImage(r, filetype)
	if err != nil {
		return errors.New("Failed to decode image: " + err.Error())
	}

	tn := resize.Thumbnail(MAX_SIZE, MAX_SIZE, img, resize.Bicubic)

	err = encodeImage(w, filetype, tn)
	if err != nil {
		return errors.New("Failed to encode image: " + err.Error())
	}

	return nil
}

// TODO Could switch to image.Decode here...
func decodeImage(reader io.Reader, filetype string) (img image.Image, err error) {
	switch filetype {
	case JPEG, JPG:
		img, err = jpeg.Decode(reader)
	case PNG:
		img, err = png.Decode(reader)
	case GIF:
		img, err = gif.Decode(reader)
	default:
		img, err = nil, errors.New("Cannot decode unknown image file type: "+filetype)
	}

	return
}

func encodeImage(w *storage.Writer, filetype string, m image.Image) (err error) {
	switch filetype {
	case JPEG, JPG:
		err = jpeg.Encode(w, m, nil)
	case PNG:
		err = png.Encode(w, m)
	case GIF:
		err = gif.Encode(w, m, nil)
	default:
		err = errors.New("Cannot encode unknown image file type: " + filetype)
	}

	return
}
