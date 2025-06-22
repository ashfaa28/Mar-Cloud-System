package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const uploadPath = "./uploads"

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Cuma boleh pake POST bang", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File tidak ditemukan", http.StatusBadRequest)
		return
	}

	defer file.Close()

	// Save file
	dst, err := os.Create(filepath.Join(uploadPath, header.Filename))
	if err != nil {
		http.Error(w, "File gabisa di save", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)
	fmt.Fprintf(w, "Upload sukses : %s\n", header.Filename)

}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Parameter file hilang", http.StatusBadRequest)
		return
	}

	filepath := filepath.Join(uploadPath, filename)
	http.ServeFile(w, r, filepath)
}

func main() {
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/upload", requireAuth(uploadHandler))
	http.HandleFunc("/download", requireAuth(downloadHandler))

	fmt.Println("Server berjalan di http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
