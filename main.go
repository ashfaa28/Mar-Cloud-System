package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	InitDB()
	startChunkCleaner(30*time.Minute, 6*time.Hour) // cek tiap 30 menit, hapus yang lebih tua 6 jam

	http.HandleFunc("/", FormHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/upload", requireAuth(UploadHandler))
	http.HandleFunc("/download", requireAuth(DownloadHandler))
	http.HandleFunc("/delete", requireAuth(DeleteHandler))
	http.HandleFunc("/list-json", requireAuth(ListJSONHandler))
	http.HandleFunc("/upload-chunk", requireAuth(UploadChunkHandler))
	http.HandleFunc("/merge", requireAuth(MergeChunksHandler))
	http.HandleFunc("/resume-status", requireAuth(ChunkStatusHandler))
	http.HandleFunc("/resume", requireAuth(ResumeUploadHandler))
	http.HandleFunc("/cancel-upload", requireAuth(CancelUploadHandler))

	http.HandleFunc("/login.html", ServeLogin)
	http.HandleFunc("/upload.html", ServeUpload)
	http.HandleFunc("/list.html", ServeList)

	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("views/js"))))

	fmt.Println("Server berjalan di http://localhost:8080")
	http.ListenAndServe("0.0.0.0:8080", nil)

}
