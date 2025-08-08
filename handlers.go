package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const uploadPath = "./uploads"
const chunkTempDir = "uploads_tmp"

// NOTE:
// - Menggunakan global variables yang sudah ada di project-mu:
//   - DB     *sql.DB
//   - jwtKey []byte (atau tipe yg kamu pakai)
//   - Claims struct (tipe klaim JWT)
// - requireAuth middleware diasumsikan tersedia dan bekerja seperti semula.

// -------------------------
// Upload chunk handler
// -------------------------
func UploadChunkHandler(w http.ResponseWriter, r *http.Request) {
	uploadID := r.FormValue("upload_id")
	chunkIndex := r.FormValue("chunk_index")
	totalChunks := r.FormValue("total_chunks")
	filename := r.FormValue("filename")

	if uploadID == "" || chunkIndex == "" || totalChunks == "" || filename == "" {
		http.Error(w, "Parameter tidak lengkap", http.StatusBadRequest)
		log.Printf("UploadChunkHandler: missing param upload_id=%q chunk_index=%q total_chunks=%q filename=%q", uploadID, chunkIndex, totalChunks, filename)
		return
	}

	// Validasi ekstensi (tambahkan .deb)
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExt := map[string]bool{
		".jpg": true, ".png": true, ".pdf": true, ".mp4": true, ".iso": true, ".deb": true,
	}
	if !allowedExt[ext] {
		http.Error(w, "Ekstensi file tidak diizinkan", http.StatusBadRequest)
		log.Printf("UploadChunkHandler: disallowed extension %s for filename %s", ext, filename)
		return
	}

	// Buat folder sementara (jika belum ada)
	chunkDir := filepath.Join(chunkTempDir, uploadID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		http.Error(w, "Gagal buat folder sementara", http.StatusInternalServerError)
		log.Printf("UploadChunkHandler: gagal mkdir %s: %v", chunkDir, err)
		return
	}

	// Ambil chunk dari form-data
	file, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "Chunk tidak ditemukan", http.StatusBadRequest)
		log.Printf("UploadChunkHandler: FormFile error: %v", err)
		return
	}
	defer file.Close()

	// Cek MIME hanya di chunk pertama (index == "0")
	if chunkIndex == "0" {
		fileHeader := make([]byte, 512)
		n, err := file.Read(fileHeader)
		if err != nil && err != io.EOF {
			http.Error(w, "Gagal baca chunk untuk validasi", http.StatusInternalServerError)
			log.Printf("UploadChunkHandler: gagal read header chunk: %v", err)
			return
		}
		filetype := http.DetectContentType(fileHeader[:n])

		allowedTypes := map[string]bool{
			"image/jpeg":                            true,
			"image/png":                             true,
			"application/pdf":                       true,
			"video/mp4":                             true,
			"application/x-iso9660-image":           true,
			"application/vnd.debian.binary-package": true,
			"application/x-debian-package":          true,
		}

		if !allowedTypes[filetype] {
			// Khusus application/octet-stream, izinkan hanya jika ekstensi .deb
			if filetype == "application/octet-stream" && ext == ".deb" {
				// Lolos, ini file .deb yang deteksi-nya generic binary
			} else {
				http.Error(w, fmt.Sprintf("Tipe file tidak diizinkan: %s", filetype), http.StatusBadRequest)
				return
			}
		}

		// Reset posisi file agar penulisan chunk tidak kehilangan data
		if seeker, ok := file.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}
	}

	// Simpan chunk ke disk
	chunkPath := filepath.Join(chunkDir, chunkIndex)
	out, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, "Gagal menyimpan chunk", http.StatusInternalServerError)
		log.Printf("UploadChunkHandler: gagal create chunk file %s: %v", chunkPath, err)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Gagal menulis chunk", http.StatusInternalServerError)
		log.Printf("UploadChunkHandler: gagal tulis chunk %s: %v", chunkPath, err)
		return
	}

	// Simpan meta.json (dipakai untuk merge & resume)
	metaPath := filepath.Join(chunkDir, "meta.json")
	meta := map[string]string{
		"filename":     filename,
		"total_chunks": totalChunks,
	}
	if metaJSON, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(metaPath, metaJSON, 0644)
	} else {
		log.Printf("UploadChunkHandler: gagal tulis meta.json untuk %s: %v", chunkDir, err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Chunk disimpan")
	log.Printf("UploadChunkHandler: saved chunk %s (uploadID=%s)", chunkPath, uploadID)
}

// -------------------------
// Merge chunks into final file
// -------------------------
func MergeChunksHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UploadID string `json:"uploadId"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		log.Printf("MergeChunksHandler: decode body error: %v", err)
		return
	}

	if req.UploadID == "" || req.Filename == "" {
		http.Error(w, "Parameter tidak lengkap", http.StatusBadRequest)
		log.Printf("MergeChunksHandler: missing params uploadId=%q filename=%q", req.UploadID, req.Filename)
		return
	}

	chunkDir := filepath.Join(chunkTempDir, req.UploadID)
	metaPath := filepath.Join(chunkDir, "meta.json")

	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		http.Error(w, "Gagal membaca meta.json", http.StatusInternalServerError)
		log.Printf("MergeChunksHandler: failed read meta.json %s: %v", metaPath, err)
		return
	}

	var meta struct {
		TotalChunks string `json:"total_chunks"`
		Filename    string `json:"filename"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		http.Error(w, "Meta.json rusak", http.StatusInternalServerError)
		log.Printf("MergeChunksHandler: unmarshal meta.json error: %v", err)
		return
	}

	totalChunks, err := strconv.Atoi(meta.TotalChunks)
	if err != nil {
		http.Error(w, "Jumlah chunk tidak valid", http.StatusInternalServerError)
		log.Printf("MergeChunksHandler: invalid total_chunks %s: %v", meta.TotalChunks, err)
		return
	}

	// Validasi ekstensi final (sama seperti UploadChunkHandler)
	ext := strings.ToLower(filepath.Ext(meta.Filename))
	allowed := map[string]bool{
		".jpg": true, ".png": true, ".pdf": true, ".mp4": true, ".iso": true, ".deb": true,
	}
	if !allowed[ext] {
		http.Error(w, "Ekstensi file tidak diizinkan", http.StatusBadRequest)
		log.Printf("MergeChunksHandler: disallowed extension %s for %s", ext, meta.Filename)
		return
	}

	// Siapkan nama target (rename otomatis jika ada)
	baseName := strings.TrimSuffix(meta.Filename, ext)
	outputFilePath := filepath.Join(uploadPath, meta.Filename)
	i := 1
	for {
		if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
			break
		}
		outputFilePath = filepath.Join(uploadPath, fmt.Sprintf("%s_%d%s", baseName, i, ext))
		i++
	}
	finalFilename := filepath.Base(outputFilePath)

	// Gabungkan semua chunk
	dst, err := os.Create(outputFilePath)
	if err != nil {
		http.Error(w, "Gagal buat file akhir", http.StatusInternalServerError)
		log.Printf("MergeChunksHandler: gagal create dst %s: %v", outputFilePath, err)
		return
	}
	defer dst.Close()

	for i := 0; i < totalChunks; i++ {
		partPath := filepath.Join(chunkDir, fmt.Sprintf("%d", i))
		part, err := os.Open(partPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal buka chunk %d", i), http.StatusInternalServerError)
			log.Printf("MergeChunksHandler: failed open part %s: %v", partPath, err)
			return
		}
		if _, err := io.Copy(dst, part); err != nil {
			part.Close()
			http.Error(w, fmt.Sprintf("Gagal tulis chunk %d", i), http.StatusInternalServerError)
			log.Printf("MergeChunksHandler: failed copy part %s: %v", partPath, err)
			return
		}
		part.Close()
	}

	// Hapus folder chunk sementara
	if err := os.RemoveAll(chunkDir); err != nil {
		log.Printf("MergeChunksHandler: warning: gagal hapus chunkDir %s: %v", chunkDir, err)
	}

	// Ambil username dari token (jika ada)
	authHeader := r.Header.Get("Authorization")
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	username := ""
	if tokenStr != "" {
		claims := &Claims{}
		if _, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		}); err == nil {
			username = claims.Username
		} else {
			log.Printf("MergeChunksHandler: token parse warning: %v", err)
		}
	}

	// Simpan metadata ke DB (jika DB tersedia)
	if DB != nil {
		if _, err := DB.Exec("INSERT INTO uploads (filename, username, uploaded_at) VALUES (?, ?, ?)", finalFilename, username, time.Now()); err != nil {
			log.Printf("MergeChunksHandler: gagal simpan metadata ke DB: %v", err)
			// tidak fatal untuk user; tetap return success merge
		}
	}

	fmt.Fprint(w, "Merge selesai!")
	log.Printf("MergeChunksHandler: merged uploadID=%s -> %s (user=%s)", req.UploadID, finalFilename, username)
}

// -------------------------
// Cancel upload (hapus chunk dir)
// -------------------------
func CancelUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UploadID string `json:"uploadId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
		http.Error(w, "uploadId diperlukan", http.StatusBadRequest)
		log.Printf("CancelUploadHandler: invalid request body: %v", err)
		return
	}

	chunkDir := filepath.Join(chunkTempDir, req.UploadID)
	if err := os.RemoveAll(chunkDir); err != nil {
		http.Error(w, "Gagal menghapus chunk", http.StatusInternalServerError)
		log.Printf("CancelUploadHandler: failed removeAll %s: %v", chunkDir, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Upload dibatalkan dan chunk dihapus")
	log.Printf("CancelUploadHandler: cancelled uploadID=%s", req.UploadID)
}

// -------------------------
// Resume upload: list chunk files already uploaded
// -------------------------
func ResumeUploadHandler(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("upload_id")
	if uploadID == "" {
		http.Error(w, "upload_id kosong", http.StatusBadRequest)
		return
	}

	chunkDir := filepath.Join(chunkTempDir, uploadID)
	// Jika folder belum ada, kembalikan array kosong (upload baru)
	if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
		json.NewEncoder(w).Encode([]string{})
		log.Printf("ResumeUploadHandler: chunkDir not exist for uploadID=%s, returning empty list", uploadID)
		return
	}

	files, err := os.ReadDir(chunkDir)
	if err != nil {
		http.Error(w, "Gagal membaca chunk", http.StatusInternalServerError)
		log.Printf("ResumeUploadHandler: ReadDir %s error: %v", chunkDir, err)
		return
	}

	var uploaded []string
	for _, f := range files {
		// Abaikan meta.json
		if strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		uploaded = append(uploaded, f.Name())
	}

	json.NewEncoder(w).Encode(uploaded)
	log.Printf("ResumeUploadHandler: uploadID=%s returned %d uploaded chunks", uploadID, len(uploaded))
}

// -------------------------
// Chunk status (mirip Resume, list yang diterima)
// -------------------------
func ChunkStatusHandler(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("upload_id")
	if uploadID == "" {
		http.Error(w, "upload_id kosong", http.StatusBadRequest)
		return
	}

	chunkDir := filepath.Join(chunkTempDir, uploadID)
	files, err := os.ReadDir(chunkDir)
	if err != nil {
		http.Error(w, "Gagal membaca folder chunk", http.StatusInternalServerError)
		log.Printf("ChunkStatusHandler: ReadDir %s error: %v", chunkDir, err)
		return
	}

	var received []string
	for _, f := range files {
		if f.Name() != "meta.json" {
			received = append(received, f.Name())
		}
	}

	json.NewEncoder(w).Encode(received)
}

// -------------------------
// Simple handlers untuk serve file HTML (login/upload/list)
// -------------------------
func FormHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/login.html")
}

func ServeLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/login.html")
}

func ServeUpload(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/upload.html")
}

func ServeList(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "views/list.html")
}

// -------------------------
// Upload biasa (non-chunked)
// -------------------------
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Cuma boleh POST", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Token tidak valid", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	}); err != nil {
		http.Error(w, "Token tidak sah", http.StatusUnauthorized)
		log.Printf("UploadHandler: token parse failed: %v", err)
		return
	}
	username := claims.Username

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File tidak ditemukan", http.StatusBadRequest)
		log.Printf("UploadHandler: FormFile error: %v", err)
		return
	}
	defer file.Close()

	filename := header.Filename
	safeName := filepath.Base(filename)

	// Rename otomatis jika sudah ada
	dstPath := filepath.Join(uploadPath, safeName)
	originalName := safeName
	i := 1
	for {
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			break
		}
		safeName = fmt.Sprintf("%s_(%d)%s", strings.TrimSuffix(originalName, filepath.Ext(originalName)), i, filepath.Ext(originalName))
		dstPath = filepath.Join(uploadPath, safeName)
		i++
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Gagal menyimpan file", http.StatusInternalServerError)
		log.Printf("UploadHandler: create dst %s error: %v", dstPath, err)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Gagal menyimpan file", http.StatusInternalServerError)
		log.Printf("UploadHandler: copy to dst %s error: %v", dstPath, err)
		return
	}

	if DB != nil {
		if _, err := DB.Exec("INSERT INTO uploads (filename, username, uploaded_at) VALUES (?, ?, ?)", safeName, username, time.Now()); err != nil {
			log.Printf("UploadHandler: gagal simpan metadata ke DB: %v", err)
		}
	}

	fmt.Fprintf(w, "Upload sukses: %s\n", safeName)
	log.Printf("UploadHandler: user=%s uploaded %s", username, safeName)
}

// -------------------------
// Download file (protected via requireAuth middleware in main.go)
// -------------------------
func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Handler ini di-wrap oleh requireAuth di main.go sehingga token sudah tervalidasi
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Parameter file kosong", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(uploadPath, filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "File tidak ditemukan", http.StatusNotFound)
		log.Printf("DownloadHandler: file not found %s", fullPath)
		return
	}

	http.ServeFile(w, r, fullPath)
	log.Printf("DownloadHandler: served %s", fullPath)
}

// -------------------------
// Delete file
// -------------------------
func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Hanya DELETE yang diizinkan", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Token tidak valid", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	}); err != nil {
		http.Error(w, "Token tidak valid", http.StatusUnauthorized)
		log.Printf("DeleteHandler: token parse failed: %v", err)
		return
	}
	username := claims.Username

	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Nama file tidak ditemukan", http.StatusBadRequest)
		return
	}

	// Hapus dari database
	if DB != nil {
		res, err := DB.Exec("DELETE FROM uploads WHERE filename = ? AND username = ?", filename, username)
		if err != nil {
			http.Error(w, "Gagal hapus dari database", http.StatusInternalServerError)
			log.Printf("DeleteHandler: DB delete error: %v", err)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			http.Error(w, "File tidak ditemukan atau bukan milikmu", http.StatusForbidden)
			log.Printf("DeleteHandler: no rows affected for %s by %s", filename, username)
			return
		}
	}

	// Hapus dari folder
	if err := os.Remove(filepath.Join(uploadPath, filename)); err != nil && !os.IsNotExist(err) {
		log.Printf("DeleteHandler: failed remove file %s: %v", filename, err)
		// tetap return success jika file sudah tidak ada
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File %s berhasil dihapus", filename)
	log.Printf("DeleteHandler: user=%s deleted %s", username, filename)
}

// -------------------------
// List JSON handler (untuk list.js) - protected by requireAuth wrapper
// -------------------------
func ListJSONHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Token tidak valid", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	}); err != nil {
		http.Error(w, "Token tidak valid", http.StatusUnauthorized)
		log.Printf("ListJSONHandler: token parse failed: %v", err)
		return
	}
	username := claims.Username

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	dateFilter := r.URL.Query().Get("date")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Build query
	query := `
		SELECT filename, uploaded_at
		FROM uploads
		WHERE username = ?
	`
	args := []interface{}{username}

	if dateFilter != "" {
		query += " AND DATE(uploaded_at) = DATE(?)"
		args = append(args, dateFilter)
	}

	query += " ORDER BY uploaded_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Gagal ambil data", http.StatusInternalServerError)
		log.Printf("ListJSONHandler: DB query error: %v", err)
		return
	}
	defer rows.Close()

	type Upload struct {
		Filename   string `json:"filename"`
		UploadedAt string `json:"uploaded_at"`
	}

	var uploads []Upload
	for rows.Next() {
		var u Upload
		if err := rows.Scan(&u.Filename, &u.UploadedAt); err != nil {
			log.Printf("ListJSONHandler: row scan error: %v", err)
			continue
		}
		uploads = append(uploads, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploads)
	log.Printf("ListJSONHandler: user=%s returned %d items (page=%d limit=%d)", username, len(uploads), page, limit)
}
