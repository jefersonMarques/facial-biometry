package imagepreview

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"faceproof/services/api/internal/document/vio/decoder"
)

type Service struct {
	bpgDecoder *BPGDecoder
}

func NewService(bpgDecoder *BPGDecoder) *Service {
	return &Service{bpgDecoder: bpgDecoder}
}

func (service *Service) Available() bool {
	return service.bpgDecoder != nil && service.bpgDecoder.Available()
}

func (service *Service) DecoderPath() string {
	if service.bpgDecoder == nil {
		return ""
	}
	return service.bpgDecoder.ResolvedPath()
}

func (service *Service) Attach(result *decoder.Result) error {
	if result == nil || len(result.Image) == 0 {
		return nil
	}

	switch strings.ToUpper(result.ImageFormat) {
	case "BPG":
		if info, err := InspectBPG(result.Image); err == nil {
			result.Technical.ImageWidth = info.Width
			result.Technical.ImageHeight = info.Height
			result.Technical.ImageBitDepth = info.BitDepth
			result.Technical.ImagePixelFormat = info.PixelFormat
			result.Technical.ImageColorSpace = info.ColorSpace
			result.Technical.ImageHasAlpha = info.HasAlpha
			result.Technical.ImageAnimation = info.Animation
		}
		if service.bpgDecoder == nil {
			return ErrDecoderUnavailable
		}
		preview, err := service.bpgDecoder.Decode(result.Image)
		if err != nil {
			return err
		}
		result.PreviewImage = preview
		result.PreviewMIME = "image/png"
		attachPreviewDimensions(result, preview)
		return nil
	case "PNG":
		result.PreviewImage = append([]byte(nil), result.Image...)
		result.PreviewMIME = "image/png"
		attachPreviewDimensions(result, result.PreviewImage)
		return nil
	case "JPEG", "JPG":
		result.PreviewImage = append([]byte(nil), result.Image...)
		result.PreviewMIME = "image/jpeg"
		attachPreviewDimensions(result, result.PreviewImage)
		return nil
	default:
		return errors.New("unsupported embedded image format")
	}
}

func attachPreviewDimensions(result *decoder.Result, data []byte) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return
	}
	result.Technical.PreviewWidth = config.Width
	result.Technical.PreviewHeight = config.Height
}
