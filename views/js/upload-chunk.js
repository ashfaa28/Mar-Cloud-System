document.addEventListener("DOMContentLoaded", () => {
  const CHUNK_SIZE = 2 * 1024 * 1024;
  let currentChunk = 0;
  let isPaused = false;
  let fileToUpload = null;
  let uploadId = null;
  let uploadedChunks = new Set();

  const fileInput = document.getElementById("chunkFileInput");
  const progressBarFile = document.getElementById("chunkProgressBar");
  const progressBarTotal = document.getElementById("totalProgressBar");
  const uploadBtn = document.getElementById("uploadBtn");
  const pauseBtn = document.getElementById("pauseBtn");
  const resumeBtn = document.getElementById("resumeBtn");

  let totalFiles = 0;
  let completedFiles = 0;

  uploadBtn.addEventListener("click", () => {
    if (!fileInput.files.length) return alert("Pilih file dulu!");

    const files = Array.from(fileInput.files);
    totalFiles = files.length;
    completedFiles = 0;
    progressBarTotal.value = 0;

    uploadMultipleFiles(files);
  });

  pauseBtn.addEventListener("click", () => {
    isPaused = true;
  });

  resumeBtn.addEventListener("click", () => {
    if (isPaused) {
      isPaused = false;
      uploadChunks();
    }
  });

  let filesQueue = [];

  function uploadMultipleFiles(files) {
    filesQueue = files;
    uploadNextFile();
  }

  function uploadNextFile() {
    if (filesQueue.length === 0) return;

    fileToUpload = filesQueue.shift();
    uploadId = `${fileToUpload.name}-${Date.now()}`;
    currentChunk = 0;
    uploadedChunks.clear();

    uploadChunks();
  }

  async function uploadChunks() {
    const token = localStorage.getItem("token");
    if (!token) return alert("Belum login!");

    const totalChunks = Math.ceil(fileToUpload.size / CHUNK_SIZE);
    progressBarFile.max = totalChunks;
    progressBarFile.value = 0;

    // Ambil daftar chunk yang sudah terupload
    try {
      const resumeRes = await fetch(`/resume?upload_id=${uploadId}`);
      const uploadedList = await resumeRes.json();
      uploadedChunks = new Set(uploadedList.map(c => c.toString()));
    } catch (err) {
      console.warn("Gagal ambil status resume, lanjut dari awal.");
    }

    while (currentChunk < totalChunks) {
      if (isPaused) return;

      if (uploadedChunks.has(currentChunk.toString())) {
        console.log(`Chunk ${currentChunk} sudah ada, skip.`);
        currentChunk++;
        progressBarFile.value = currentChunk;
        continue;
      }

      const start = currentChunk * CHUNK_SIZE;
      const end = Math.min(fileToUpload.size, start + CHUNK_SIZE);
      const chunk = fileToUpload.slice(start, end);

      const formData = new FormData();
      formData.append("chunk", chunk);
      formData.append("chunk_index", currentChunk);
      formData.append("total_chunks", totalChunks);
      formData.append("upload_id", uploadId);
      formData.append("filename", fileToUpload.name);

      try {
        const res = await fetch("/upload-chunk", {
          method: "POST",
          headers: {
            "Authorization": "Bearer " + token,
          },
          body: formData,
        });

        const result = await res.text();
        console.log(`Chunk ${currentChunk}: ${result}`);
      } catch (err) {
        console.error(`Chunk ${currentChunk} gagal diupload:`, err);
        return;
      }

      currentChunk++;
      progressBarFile.value = currentChunk;
    }

    // Jika semua chunk selesai, merge
    try {
      const res = await fetch("/merge", {
        method: "POST",
        headers: {
          "Authorization": "Bearer " + token,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          uploadId,
          filename: fileToUpload.name,
        }),
      });

      const msg = await res.text();
      console.log("Merge berhasil:", msg);
    } catch (err) {
      console.error("Gagal merge:", err);
      alert("Gagal merge file!");
      return;
    }

    // Satu file selesai
    completedFiles++;
    progressBarTotal.value = (completedFiles / totalFiles) * 100;

    // Lanjut file berikutnya
    uploadNextFile();
  }
});
