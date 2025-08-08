package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

func startChunkCleaner(interval time.Duration, maxAge time.Duration) {
	go func() {
		for {
			filepath.Walk(chunkTempDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil {
					return nil
				}
				// Hanya hapus direktori per-upload; jika info.IsDir() dan bukan root
				if info.IsDir() && path != chunkTempDir {
					if time.Since(info.ModTime()) > maxAge {
						log.Println("Cleaner: menghapus", path)
						os.RemoveAll(path)
					}
				}
				return nil
			})
			time.Sleep(interval)
		}
	}()
}
