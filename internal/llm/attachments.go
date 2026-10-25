package llm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxAttachmentBytes = 512 << 10
const MaxAttachmentTotalBytes = 1 << 20
const PCM16MIME = "audio/pcm;rate=16000"

// Attachment contains explicitly selected bytes, never a provider-fetch URL.
// Payloads belong in private artifacts, not metadata or diagnostics.
type Attachment struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data []byte `json:"data"`
}

func (a Attachment) Validate() error {
	if a.Name == "" || len(a.Name) > 128 || !utf8.ValidString(a.Name) || strings.TrimSpace(a.Name) != a.Name || strings.ContainsAny(a.Name, "/\\:") || strings.IndexFunc(a.Name, unicode.IsControl) >= 0 {
		return fmt.Errorf("attachment requires a plain display name up to 128 bytes")
	}
	if len(a.Data) == 0 || len(a.Data) > MaxAttachmentBytes {
		return fmt.Errorf("attachment must contain between 1 byte and 512 KiB")
	}
	switch a.MIME {
	case "image/png":
		return validateAttachmentImage(a.Data, png.DecodeConfig, png.Decode)
	case "image/jpeg":
		return validateAttachmentImage(a.Data, jpeg.DecodeConfig, jpeg.Decode)
	case "application/pdf":
		return validateAttachmentPDF(a.Data)
	case "audio/wav":
		return validateAttachmentWAV(a.Data)
	case PCM16MIME:
		if len(a.Data)%2 != 0 {
			return fmt.Errorf("PCM audio requires complete signed 16-bit little-endian samples")
		}
		return nil
	default:
		return fmt.Errorf("unsupported attachment MIME; use PNG, JPEG, PDF, 16 kHz mono PCM WAV, or audio/pcm;rate=16000")
	}
}

func ValidateAttachments(items []Attachment) error {
	if len(items) < 1 || len(items) > 4 {
		return fmt.Errorf("select between one and four attachments")
	}
	total := 0
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return err
		}
		total += len(item.Data)
	}
	if total > MaxAttachmentTotalBytes {
		return fmt.Errorf("selected attachments exceed 1 MiB combined")
	}
	return nil
}

func ValidateAdviceAttachments(items []Attachment) error {
	if err := ValidateAttachments(items); err != nil {
		return err
	}
	for _, item := range items {
		if item.MIME == PCM16MIME {
			return fmt.Errorf("raw PCM is live-session input; use a WAV attachment for completion advice")
		}
	}
	return nil
}

func validateAttachmentImage(data []byte, config func(io.Reader) (image.Config, error), decode func(io.Reader) (image.Image, error)) error {
	info, err := config(bytes.NewReader(data))
	if err != nil || info.Width < 1 || info.Height < 1 || info.Width > 4096 || info.Height > 4096 || int64(info.Width)*int64(info.Height) > 4<<20 {
		return fmt.Errorf("invalid attachment image or dimensions exceed 4096 per side / 4 megapixels")
	}
	if _, err := decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("attachment image content is incomplete or malformed")
	}
	return nil
}

func validateAttachmentPDF(data []byte) error {
	first, _, found := bytes.Cut(data, []byte("\n"))
	first = bytes.TrimSuffix(first, []byte("\r"))
	validVersion := bytes.Equal(first, []byte("%PDF-2.0")) || len(first) == 8 && bytes.HasPrefix(first, []byte("%PDF-1.")) && first[7] >= '0' && first[7] <= '7'
	if !found || !validVersion || len(data) < 24 || !bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF")) {
		return fmt.Errorf("attachment PDF requires a supported header and complete EOF envelope")
	}
	return nil
}

func validateAttachmentWAV(data []byte) error {
	if !validAttachmentWAVHeader(data) {
		return fmt.Errorf("invalid WAV attachment envelope")
	}
	format, samples := false, false
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return fmt.Errorf("incomplete WAV attachment chunk")
		}
		name := string(data[offset : offset+4])
		size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		end := uint64(offset) + 8 + size
		if end+size%2 > uint64(len(data)) {
			return fmt.Errorf("incomplete WAV attachment payload")
		}
		chunk := data[offset+8 : int(end)]
		switch name {
		case "fmt ":
			if format || !validAttachmentWAVFormat(chunk) {
				return fmt.Errorf("WAV attachment requires uncompressed 16 kHz mono signed 16-bit PCM")
			}
			format = true
		case "data":
			if samples || !format || len(chunk) == 0 || len(chunk)%2 != 0 {
				return fmt.Errorf("invalid WAV attachment samples")
			}
			samples = true
		}
		offset = int(end + size%2)
	}
	if !format || !samples {
		return fmt.Errorf("WAV attachment is missing format or sample data")
	}
	return nil
}

func validAttachmentWAVHeader(data []byte) bool {
	return len(data) >= 44 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" && uint64(binary.LittleEndian.Uint32(data[4:8]))+8 == uint64(len(data))
}

func validAttachmentWAVFormat(data []byte) bool {
	return len(data) == 16 && binary.LittleEndian.Uint16(data) == 1 && binary.LittleEndian.Uint16(data[2:]) == 1 && binary.LittleEndian.Uint32(data[4:]) == 16000 && binary.LittleEndian.Uint32(data[8:]) == 32000 && binary.LittleEndian.Uint16(data[12:]) == 2 && binary.LittleEndian.Uint16(data[14:]) == 16
}
