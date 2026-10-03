package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

type indexContent struct {
	hash   string
	size   int64
	lines  int
	binary bool
	source []byte
}

// Keep at most the editor's supported source size for parsing, while hashes and
// line counts cover the entire file. Binary assets need only the detection sample.
func readIndexContent(path string, info os.FileInfo) (indexContent, error) {
	if !info.Mode().IsRegular() {
		return indexContent{}, fmt.Errorf("source is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return indexContent{}, err
	}
	defer file.Close()
	result := indexContent{}
	if info.Size() <= maxFileViewBytes {
		result.source = make([]byte, 0, info.Size())
	}
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	var last byte
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			_, _ = hash.Write(chunk)
			result.size += int64(n)
			result.lines += bytes.Count(chunk, []byte{'\n'})
			last = chunk[n-1]
			if len(result.source) < maxFileViewBytes && !result.binary {
				keep := min(n, maxFileViewBytes-len(result.source))
				result.source = append(result.source, chunk[:keep]...)
				if len(result.source) >= 8192 {
					result.binary = isBinary(result.source)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return indexContent{}, err
		}
	}
	result.binary = isBinary(result.source)
	if result.size > 0 && last != '\n' {
		result.lines++
	}
	result.hash = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return result, nil
}
