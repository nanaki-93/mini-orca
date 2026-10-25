package llm

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"slices"
	"strings"
	"testing"
)

func attachmentWAV() []byte {
	data := make([]byte, 48)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 16000)
	binary.LittleEndian.PutUint32(data[28:], 32000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], 4)
	return data
}

func TestAttachmentContentValidation(t *testing.T) {
	var pngData, jpegData bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&pngData, picture); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, picture, nil); err != nil {
		t.Fatal(err)
	}
	valid := []Attachment{
		{Name: "diagram.png", MIME: "image/png", Data: pngData.Bytes()},
		{Name: "photo.jpg", MIME: "image/jpeg", Data: jpegData.Bytes()},
		{Name: "brief.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF\n")},
		{Name: "sample.wav", MIME: "audio/wav", Data: attachmentWAV()},
		{Name: "sample.pcm", MIME: PCM16MIME, Data: []byte{0, 1, 2, 3}},
	}
	for _, item := range valid {
		if err := item.Validate(); err != nil {
			t.Fatalf("valid %s: %v", item.MIME, err)
		}
	}
	if err := ValidateAdviceAttachments(valid[:4]); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAdviceAttachments(valid[4:]); err == nil {
		t.Fatal("raw PCM advertised as completion audio")
	}
	for _, name := range []string{"", "../image.png", "https://private.example/file", "a\\b", "bad\x1b.png", " spaced.png", strings.Repeat("n", 129), "\xff"} {
		item := valid[0]
		item.Name = name
		if item.Validate() == nil {
			t.Fatalf("invalid display name accepted: %q", name)
		}
	}
	for _, kind := range []string{"mismatch", "truncated png", "truncated jpeg", "oversized", "empty", "pdf header", "pdf end", "pcm sample", "pcm rate", "svg"} {
		t.Run(kind, func(t *testing.T) {
			item := valid[0]
			switch kind {
			case "mismatch":
				item.MIME = "image/jpeg"
			case "truncated png":
				item.Data = item.Data[:40]
			case "truncated jpeg":
				item = valid[1]
				item.Data = item.Data[:len(item.Data)-20]
			case "oversized":
				item.Data = make([]byte, MaxAttachmentBytes+1)
			case "empty":
				item.Data = nil
			case "pdf header":
				item = valid[2]
				item.Data = []byte("%PDF-8.1\n1 0 obj\n<<>>\nendobj\n%%EOF")
			case "pdf end":
				item = valid[2]
				item.Data = bytes.TrimSuffix(item.Data, []byte("%%EOF\n"))
			case "pcm sample":
				item = valid[4]
				item.Data = []byte{1}
			case "pcm rate":
				item = valid[4]
				item.MIME = "audio/pcm;rate=48000"
			case "svg":
				item.MIME = "image/svg+xml"
				item.Data = []byte("<svg/>")
			}
			if item.Validate() == nil {
				t.Fatal("invalid media accepted")
			}
		})
	}
}

func TestAttachmentWAVRejectsAmbiguousOrIncompleteChunks(t *testing.T) {
	for _, kind := range []string{"header", "length", "format", "rate", "missing data", "oversized chunk", "partial chunk", "odd samples", "duplicate format"} {
		t.Run(kind, func(t *testing.T) {
			data := attachmentWAV()
			switch kind {
			case "header":
				data[0] = 'X'
			case "length":
				data[4] = 0
			case "format":
				data[20] = 3
			case "rate":
				data[24] = 1
			case "missing data":
				copy(data[36:], "JUNK")
			case "oversized chunk":
				binary.LittleEndian.PutUint32(data[40:], 1<<31)
			case "partial chunk":
				data = append(data, 0)
				binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
			case "odd samples":
				data[40] = 3
			case "duplicate format":
				data = append(data, slices.Clone(data[12:36])...)
				binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
			}
			if (Attachment{Name: "clip.wav", MIME: "audio/wav", Data: data}).Validate() == nil {
				t.Fatal("invalid WAV accepted")
			}
		})
	}
}

func TestAttachmentAggregateBounds(t *testing.T) {
	item := Attachment{Name: "clip.pcm", MIME: PCM16MIME, Data: make([]byte, MaxAttachmentBytes)}
	if ValidateAttachments([]Attachment{item, item}) != nil {
		t.Fatal("exact aggregate bound rejected")
	}
	for _, items := range [][]Attachment{nil, {item, item, item}, {item, item, item, item, item}} {
		if ValidateAttachments(items) == nil {
			t.Fatal("unbounded selection accepted")
		}
	}
	var large bytes.Buffer
	if err := png.Encode(&large, image.NewRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	if (Attachment{Name: "wide.png", MIME: "image/png", Data: large.Bytes()}).Validate() == nil {
		t.Fatal("oversized dimensions accepted")
	}
}
