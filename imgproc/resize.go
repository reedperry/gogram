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

const TN_SIZE uint = 100
const VIEW_SIZE uint = 1024
const JPEG = "image/jpeg"
const JPG = "image/jpg"
const PNG = "image/png"
const GIF = "image/gif"

type resizer interface {
	Resize(string, io.Reader, *storage.Writer) error
	Filename(string) string
}

type ThumbnailSizer struct{}
type ViewSizer struct{}

func (t *ThumbnailSizer) Resize(filetype string, r io.Reader, w *storage.Writer) error {
	return createSizedCopy(TN_SIZE, filetype, r, w)
}

func (t *ThumbnailSizer) Filename(filename string) string {
	return filename + "_thumb"
}

func (v *ViewSizer) Resize(filetype string, r io.Reader, w *storage.Writer) error {
	return createSizedCopy(VIEW_SIZE, filetype, r, w)
}

func (v *ViewSizer) Filename(filename string) string {
	return filename + "_view"
}

func createSizedCopy(maxSize uint, filetype string, r io.Reader, w *storage.Writer) error {
	img, err := decodeImage(r, filetype)
	if err != nil {
		return errors.New("Failed to decode image: " + err.Error())
	}

	tn := resize.Thumbnail(maxSize, maxSize, img, resize.Bicubic)

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
