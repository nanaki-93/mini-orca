package project

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkIndexLargeBinary(b *testing.B) {
	root := b.TempDir()
	data := make([]byte, 16*1024*1024)
	if err := os.WriteFile(filepath.Join(root, "asset.bin"), data, 0600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildIndexFile(root, "asset.bin", IndexFile{}); err != nil {
			b.Fatal(err)
		}
	}
}
