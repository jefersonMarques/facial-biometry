package pdfanalysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxEmbeddedImages = 64

type ImageAnalysis struct {
	Count                         int                     `json:"count"`
	ProfileVersion                string                  `json:"profile_version"`
	StructuralSHA256              string                  `json:"structural_sha256,omitempty"`
	ObjectGraphSHA256             string                  `json:"object_graph_sha256,omitempty"`
	FrontCandidateIndex           int                     `json:"front_candidate_index"`
	FrontCandidateDetected        bool                    `json:"front_candidate_detected"`
	StandalonePortraitCandidates int                     `json:"standalone_portrait_candidates"`
	SquareJPEGCandidates          int                     `json:"square_jpeg_candidates"`
	EXIFImageCount                int                     `json:"exif_image_count"`
	PhotoshopImageCount           int                     `json:"photoshop_image_count"`
	XMPImageCount                 int                     `json:"xmp_image_count"`
	Images                        []EmbeddedImageEvidence `json:"images,omitempty"`
}

type EmbeddedImageEvidence struct {
	Index            int           `json:"index"`
	Page             int           `json:"page"`
	Type             string        `json:"type,omitempty"`
	Width            int           `json:"width,omitempty"`
	Height           int           `json:"height,omitempty"`
	ColorSpace       string        `json:"color_space,omitempty"`
	Components       int           `json:"components,omitempty"`
	BitsPerComponent int           `json:"bits_per_component,omitempty"`
	Encoding         string        `json:"encoding,omitempty"`
	Interpolation    string        `json:"interpolation,omitempty"`
	ObjectID         int           `json:"object_id,omitempty"`
	ObjectGeneration int           `json:"object_generation,omitempty"`
	XPPI             float64       `json:"x_ppi,omitempty"`
	YPPI             float64       `json:"y_ppi,omitempty"`
	ReportedSize     string        `json:"reported_size,omitempty"`
	ReportedRatio    string        `json:"reported_ratio,omitempty"`
	MIME             string        `json:"mime,omitempty"`
	ExtractedBytes   int           `json:"extracted_bytes,omitempty"`
	ContentSHA256          string        `json:"content_sha256,omitempty"`
	RawStreamSHA256        string        `json:"raw_stream_sha256,omitempty"`
	ObjectDictionarySHA256 string        `json:"object_dictionary_sha256,omitempty"`
	JPEG                   *JPEGMetadata `json:"jpeg,omitempty"`
}

type JPEGMetadata struct {
	JFIF                    bool   `json:"jfif"`
	JFIFVersion             string `json:"jfif_version,omitempty"`
	DensityUnit             int    `json:"density_unit,omitempty"`
	XDensity                int    `json:"x_density,omitempty"`
	YDensity                int    `json:"y_density,omitempty"`
	EXIFPresent             bool   `json:"exif_present"`
	XMPPresent              bool   `json:"xmp_present"`
	ICCProfilePresent       bool   `json:"icc_profile_present"`
	PhotoshopAPP13Present   bool   `json:"photoshop_app13_present"`
	AdobeAPP14Present       bool   `json:"adobe_app14_present"`
	Progressive             bool   `json:"progressive"`
	QuantizationSHA256      string `json:"quantization_sha256,omitempty"`
	HuffmanSHA256           string `json:"huffman_sha256,omitempty"`
	MarkerProfileSHA256     string `json:"marker_profile_sha256,omitempty"`
}

func analyzeEmbeddedImages(parent context.Context, pdfPath string, pageNumber int, wantCNHPhoto bool) (ImageAnalysis, Photo, error) {
	binary, err := exec.LookPath("pdfimages")
	if err != nil {
		return ImageAnalysis{}, Photo{}, exec.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()

	listCmd := exec.CommandContext(ctx, binary, "-f", strconv.Itoa(pageNumber), "-l", strconv.Itoa(pageNumber), "-list", pdfPath)
	listCmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	listOutput, err := listCmd.CombinedOutput()
	if ctx.Err() != nil {
		return ImageAnalysis{}, Photo{}, ctx.Err()
	}
	if err != nil {
		return ImageAnalysis{}, Photo{}, fmt.Errorf("pdfimages -list: %v: %s", err, strings.TrimSpace(string(listOutput)))
	}

	rows := parsePDFImagesList(string(listOutput))
	totalRows := len(rows)
	if len(rows) > maxEmbeddedImages {
		rows = rows[:maxEmbeddedImages]
	}

	dir, err := os.MkdirTemp("", "faceproof-pdf-images-*")
	if err != nil {
		return ImageAnalysis{}, Photo{}, err
	}
	defer os.RemoveAll(dir)

	prefix := filepath.Join(dir, "image")
	extractCtx, extractCancel := context.WithTimeout(parent, commandTimeout)
	defer extractCancel()
	extractCmd := exec.CommandContext(
		extractCtx,
		binary,
		"-f", strconv.Itoa(pageNumber),
		"-l", strconv.Itoa(pageNumber),
		"-all",
		pdfPath,
		prefix,
	)
	extractCmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	if output, err := extractCmd.CombinedOutput(); err != nil {
		if extractCtx.Err() != nil {
			return ImageAnalysis{}, Photo{}, extractCtx.Err()
		}
		return ImageAnalysis{}, Photo{}, fmt.Errorf("pdfimages -all: %v: %s", err, strings.TrimSpace(string(output)))
	}

	files, _ := filepath.Glob(prefix + "-*")
	sort.Strings(files)

	analysis := ImageAnalysis{
		Count:               totalRows,
		ProfileVersion:      "pdfimages-v1",
		FrontCandidateIndex: -1,
		Images:              rows,
	}

	for i := range analysis.Images {
		if i >= len(files) {
			break
		}
		data, err := os.ReadFile(files[i])
		if err != nil {
			continue
		}
		analysis.Images[i].ExtractedBytes = len(data)
		sum := sha256.Sum256(data)
		analysis.Images[i].ContentSHA256 = hex.EncodeToString(sum[:])
		analysis.Images[i].MIME = mimeFromImageFile(files[i], data)

		if strings.EqualFold(analysis.Images[i].Encoding, "jpeg") || analysis.Images[i].MIME == "image/jpeg" {
			meta := parseJPEGMetadata(data)
			analysis.Images[i].JPEG = &meta
		}
	}

	if pdfData, err := os.ReadFile(pdfPath); err == nil {
		attachRawPDFObjectHashes(pdfData, analysis.Images)
	}

	analysis.StructuralSHA256 = imageStructureFingerprint(analysis.Images, false)
	analysis.ObjectGraphSHA256 = imageStructureFingerprint(analysis.Images, true)
	analysis.StandalonePortraitCandidates = countStandalonePortraitCandidates(analysis.Images)
	analysis.SquareJPEGCandidates = countSquareJPEGCandidates(analysis.Images)
	for _, item := range analysis.Images {
		if item.JPEG == nil {
			continue
		}
		if item.JPEG.EXIFPresent {
			analysis.EXIFImageCount++
		}
		if item.JPEG.PhotoshopAPP13Present {
			analysis.PhotoshopImageCount++
		}
		if item.JPEG.XMPPresent {
			analysis.XMPImageCount++
		}
	}

	candidate := selectCNHFrontCandidate(analysis.Images)
	var photo Photo
	if candidate >= 0 {
		analysis.FrontCandidateDetected = true
		analysis.FrontCandidateIndex = candidate
		if wantCNHPhoto && candidate < len(files) {
			if data, err := os.ReadFile(files[candidate]); err == nil {
				if cropped, err := cropCNHFrontPortrait(data, pageNumber); err == nil {
					photo = cropped
				}
			}
		}
	}
	return analysis, photo, nil
}

func parsePDFImagesList(output string) []EmbeddedImageEvidence {
	var rows []EmbeddedImageEvidence
	for _, raw := range strings.Split(output, "\n") {
		fields := strings.Fields(raw)
		if len(fields) < 16 {
			continue
		}
		page, errPage := strconv.Atoi(fields[0])
		num, errNum := strconv.Atoi(fields[1])
		width, errWidth := strconv.Atoi(fields[3])
		height, errHeight := strconv.Atoi(fields[4])
		components, errComp := strconv.Atoi(fields[6])
		bpc, errBPC := strconv.Atoi(fields[7])
		objectID, errObj := strconv.Atoi(fields[10])
		objectGen, errGen := strconv.Atoi(fields[11])
		xppi, errX := strconv.ParseFloat(fields[12], 64)
		yppi, errY := strconv.ParseFloat(fields[13], 64)
		if errPage != nil || errNum != nil || errWidth != nil || errHeight != nil || errComp != nil || errBPC != nil || errObj != nil || errGen != nil || errX != nil || errY != nil {
			continue
		}
		rows = append(rows, EmbeddedImageEvidence{
			Index:            num,
			Page:             page,
			Type:             fields[2],
			Width:            width,
			Height:           height,
			ColorSpace:       fields[5],
			Components:       components,
			BitsPerComponent: bpc,
			Encoding:         fields[8],
			Interpolation:    fields[9],
			ObjectID:         objectID,
			ObjectGeneration: objectGen,
			XPPI:             xppi,
			YPPI:             yppi,
			ReportedSize:     fields[14],
			ReportedRatio:    fields[15],
		})
	}
	return rows
}

func attachRawPDFObjectHashes(pdf []byte, images []EmbeddedImageEvidence) {
	for i := range images {
		if images[i].ObjectID <= 0 {
			continue
		}
		header := []byte(fmt.Sprintf("%d %d obj", images[i].ObjectID, images[i].ObjectGeneration))
		objectStart := bytes.Index(pdf, header)
		if objectStart < 0 {
			continue
		}
		objectBody := pdf[objectStart+len(header):]
		streamPos := bytes.Index(objectBody, []byte("stream"))
		if streamPos < 0 {
			continue
		}
		dictionary := bytes.TrimSpace(objectBody[:streamPos])
		if len(dictionary) > 0 {
			sum := sha256.Sum256(dictionary)
			images[i].ObjectDictionarySHA256 = hex.EncodeToString(sum[:])
		}

		streamStart := streamPos + len("stream")
		if streamStart < len(objectBody) && objectBody[streamStart] == '\r' {
			streamStart++
		}
		if streamStart < len(objectBody) && objectBody[streamStart] == '\n' {
			streamStart++
		}
		endPos := bytes.Index(objectBody[streamStart:], []byte("endstream"))
		if endPos < 0 {
			continue
		}
		raw := objectBody[streamStart : streamStart+endPos]
		if len(raw) > 0 && raw[len(raw)-1] == '\n' {
			raw = raw[:len(raw)-1]
			if len(raw) > 0 && raw[len(raw)-1] == '\r' {
				raw = raw[:len(raw)-1]
			}
		}
		if len(raw) == 0 {
			continue
		}
		sum := sha256.Sum256(raw)
		images[i].RawStreamSHA256 = hex.EncodeToString(sum[:])
	}
}

func imageStructureFingerprint(images []EmbeddedImageEvidence, includeObjects bool) string {
	if len(images) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, item := range images {
		fmt.Fprintf(&builder, "%d|%s|%d|%d|%s|%d|%d|%s|%s|%.3f|%.3f",
			item.Page,
			strings.ToLower(item.Type),
			item.Width,
			item.Height,
			strings.ToLower(item.ColorSpace),
			item.Components,
			item.BitsPerComponent,
			strings.ToLower(item.Encoding),
			strings.ToLower(item.Interpolation),
			item.XPPI,
			item.YPPI,
		)
		if includeObjects {
			fmt.Fprintf(&builder, "|%d|%d", item.ObjectID, item.ObjectGeneration)
		}
		if item.JPEG != nil {
			fmt.Fprintf(&builder, "|%s|%s|%s",
				item.JPEG.QuantizationSHA256,
				item.JPEG.HuffmanSHA256,
				item.JPEG.MarkerProfileSHA256,
			)
		}
		builder.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func countStandalonePortraitCandidates(images []EmbeddedImageEvidence) int {
	count := 0
	for _, item := range images {
		if strings.ToLower(item.Type) != "image" || item.Width < 500 || item.Height < 700 {
			continue
		}
		ratio := float64(item.Width) / float64(item.Height)
		if ratio >= 0.55 && ratio <= 0.90 {
			count++
		}
	}
	return count
}

func countSquareJPEGCandidates(images []EmbeddedImageEvidence) int {
	count := 0
	for _, item := range images {
		if strings.ToLower(item.Type) != "image" || !strings.EqualFold(item.Encoding, "jpeg") {
			continue
		}
		if item.Width < 300 || item.Height < 300 {
			continue
		}
		ratio := float64(item.Width) / float64(item.Height)
		if ratio >= 0.90 && ratio <= 1.10 {
			count++
		}
	}
	return count
}

func selectCNHFrontCandidate(images []EmbeddedImageEvidence) int {
	best := -1
	bestPixels := 0
	for i, item := range images {
		if strings.ToLower(item.Type) != "image" {
			continue
		}
		if item.Width < 600 || item.Height < 400 {
			continue
		}
		ratio := float64(item.Width) / float64(item.Height)
		if ratio < 1.20 || ratio > 1.60 {
			continue
		}
		pixels := item.Width * item.Height
		if pixels > bestPixels {
			best = i
			bestPixels = pixels
		}
	}
	return best
}

func cropCNHFrontPortrait(data []byte, pageNumber int) (Photo, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Photo{}, err
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width < 600 || height < 400 {
		return Photo{}, errors.New("imagem frontal pequena demais")
	}

	x1 := b.Min.X + int(float64(width)*0.18)
	x2 := b.Min.X + int(float64(width)*0.37)
	y1 := b.Min.Y + int(float64(height)*0.38)
	y2 := b.Min.Y + int(float64(height)*0.78)
	if x2 <= x1 || y2 <= y1 || x2 > b.Max.X || y2 > b.Max.Y {
		return Photo{}, errors.New("recorte da foto fora da imagem")
	}

	dst := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: x1, Y: y1}, draw.Src)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, dst); err != nil {
		return Photo{}, err
	}
	photoBytes := encoded.Bytes()
	sum := sha256.Sum256(photoBytes)
	return Photo{
		Available:  true,
		Page:       pageNumber,
		Method:     "pdfimages_structural_candidate_crop_v1",
		Confidence: "layout",
		MIME:       "image/png",
		Width:      x2 - x1,
		Height:     y2 - y1,
		SHA256:     hex.EncodeToString(sum[:]),
		Bytes:      photoBytes,
	}, nil
}

func mimeFromImageFile(path string, data []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".jp2", ".j2k":
		return "image/jp2"
	case ".pbm", ".pgm", ".ppm":
		return "image/x-portable-anymap"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	return "application/octet-stream"
}

func parseJPEGMetadata(data []byte) JPEGMetadata {
	meta := JPEGMetadata{}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return meta
	}

	var quantization bytes.Buffer
	var huffman bytes.Buffer
	var markerProfile strings.Builder

	for offset := 2; offset+1 < len(data); {
		if data[offset] != 0xff {
			offset++
			continue
		}
		for offset < len(data) && data[offset] == 0xff {
			offset++
		}
		if offset >= len(data) {
			break
		}
		marker := data[offset]
		offset++

		if marker == 0xd9 || marker == 0xda {
			break
		}
		if marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if offset+2 > len(data) {
			break
		}
		length := int(data[offset])<<8 | int(data[offset+1])
		if length < 2 || offset+length > len(data) {
			break
		}
		payload := data[offset+2 : offset+length]
		fmt.Fprintf(&markerProfile, "%02x:%d;", marker, len(payload))

		switch marker {
		case 0xe0:
			if len(payload) >= 12 && string(payload[:5]) == "JFIF\x00" {
				meta.JFIF = true
				meta.JFIFVersion = fmt.Sprintf("%d.%02d", payload[5], payload[6])
				meta.DensityUnit = int(payload[7])
				meta.XDensity = int(payload[8])<<8 | int(payload[9])
				meta.YDensity = int(payload[10])<<8 | int(payload[11])
			}
		case 0xe1:
			if len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
				meta.EXIFPresent = true
			}
			if bytes.Contains(payload, []byte("http://ns.adobe.com/xap/1.0/")) {
				meta.XMPPresent = true
			}
		case 0xe2:
			if len(payload) >= 12 && string(payload[:12]) == "ICC_PROFILE\x00" {
				meta.ICCProfilePresent = true
			}
		case 0xed:
			if bytes.Contains(payload, []byte("Photoshop 3.0")) || bytes.Contains(payload, []byte("8BIM")) {
				meta.PhotoshopAPP13Present = true
			}
		case 0xee:
			if bytes.HasPrefix(payload, []byte("Adobe")) {
				meta.AdobeAPP14Present = true
			}
		case 0xdb:
			quantization.Write(payload)
		case 0xc4:
			huffman.Write(payload)
		case 0xc2:
			meta.Progressive = true
		}
		offset += length
	}

	if quantization.Len() > 0 {
		sum := sha256.Sum256(quantization.Bytes())
		meta.QuantizationSHA256 = hex.EncodeToString(sum[:])
	}
	if huffman.Len() > 0 {
		sum := sha256.Sum256(huffman.Bytes())
		meta.HuffmanSHA256 = hex.EncodeToString(sum[:])
	}
	if markerProfile.Len() > 0 {
		sum := sha256.Sum256([]byte(markerProfile.String()))
		meta.MarkerProfileSHA256 = hex.EncodeToString(sum[:])
	}
	return meta
}

func runWithTimeout(parent context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
}
