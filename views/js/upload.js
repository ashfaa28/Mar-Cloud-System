document.addEventListener("DOMContentLoaded", () => {

  // ==========================================================
  // BAGIAN 1: UPLOAD BIASA (MULTI FILE)
  // ==========================================================
  const multiFileInput = document.getElementById("multiFileInput");
  const dropZone = document.getElementById("dropZone");
  const fileList = document.getElementById("fileList");
  const multiUploadBtn = document.getElementById("multiUploadBtn");
  const totalProgress = document.getElementById("totalProgress");
  const totalProgressBar = document.getElementById("totalProgressBar");

  let filesToUpload = [];

  dropZone.addEventListener("click", () => multiFileInput.click());
  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("hover");
  });
  dropZone.addEventListener("dragleave", () => dropZone.classList.remove("hover"));
  dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropZone.classList.remove("hover");
    handleFiles(e.dataTransfer.files);
  });
  multiFileInput.addEventListener("change", (e) => handleFiles(e.target.files));

  function handleFiles(selectedFiles) {
    for (let file of selectedFiles) {
      if (file.size > 2 * 1024 * 1024 * 1024) { // 2 GB
        alert(`File ${file.name} terlalu besar (max 2GB).`);
        continue;
      }
      if (!["image/jpeg", "image/png", "application/pdf", "video/mp4", "application/x-iso9660-image"].includes(file.type)) {
        alert(`File ${file.name} tidak diizinkan.`);
        continue;
      }

      let renamed = file.name;
      filesToUpload.push({ file, renamed });

      let item = document.createElement("div");
      item.classList.add("file-item");
      item.innerHTML = `
        ${file.name}
        <input type="text" class="rename-input" value="${file.name}">
        <progress class="file-progress" value="0" max="100"></progress>
        <button class="cancel-btn">Batal</button>
      `;

      item.querySelector(".rename-input").addEventListener("input", (e) => {
        filesToUpload.find(f => f.file === file).renamed = e.target.value;
      });

      item.querySelector(".cancel-btn").addEventListener("click", () => {
        filesToUpload = filesToUpload.filter(f => f.file !== file);
        item.remove();
        toggleUploadButton();
      });

      fileList.appendChild(item);
    }
    toggleUploadButton();
  }

  function toggleUploadButton() {
    multiUploadBtn.disabled = filesToUpload.length === 0;
  }

  multiUploadBtn.addEventListener("click", async () => {
    const token = localStorage.getItem("token");
    if (!token) return alert("Belum login!");

    multiUploadBtn.disabled = true;
    totalProgressBar.value = 0;
    totalProgress.style.display = "block";

    let totalUploaded = 0;
    let totalSize = filesToUpload.reduce((sum, f) => sum + f.file.size, 0);

    for (let i = 0; i < filesToUpload.length; i++) {
      let { file, renamed } = filesToUpload[i];
      await uploadFile(file, renamed, token, (progress) => {
        totalUploaded += (file.size * progress) / 100;
        totalProgressBar.value = Math.floor((totalUploaded / totalSize) * 100);
        document.querySelectorAll(".file-progress")[i].value = progress;
      });
    }

    alert("Semua file berhasil diupload!");
    location.reload();
  });

  function uploadFile(file, newName, token, onProgress) {
    return new Promise((resolve, reject) => {
      let xhr = new XMLHttpRequest();
      let formData = new FormData();
      formData.append("file", file);
      formData.append("rename", newName);

      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable) {
          let percent = Math.floor((e.loaded / e.total) * 100);
          onProgress(percent);
        }
      });

      xhr.open("POST", "/upload");
      xhr.setRequestHeader("Authorization", "Bearer " + token);
      xhr.onload = () => resolve();
      xhr.onerror = () => reject();
      xhr.send(formData);
    });
  }

  // ==========================================================
  // BAGIAN 2: UPLOAD CHUNKED (FILE JUMBO)
  // ==========================================================
  const CHUNK_SIZE = 2 * 1024 * 1024;
  let currentChunk = 0;
  let isPaused = false;
  let fileToUpload = null;
  let uploadId = null;
  let uploadedChunks = new Set();
  let filesQueue = [];
  let totalFiles = 0;
  let completedFiles = 0;

  const chunkFileInput = document.getElementById("chunkFileInput");
  const chunkProgressBar = document.getElementById("chunkProgressBar");
  const chunkTotalProgressBar = document.getElementById("chunkTotalProgressBar");
  const chunkUploadBtn = document.getElementById("chunkUploadBtn");
  const pauseBtn = document.getElementById("pauseBtn");
  const resumeBtn = document.getElementById("resumeBtn");

  chunkUploadBtn.addEventListener("click", () => {
    if (!chunkFileInput.files.length) return alert("Pilih file dulu!");

    const files = Array.from(chunkFileInput.files);
    totalFiles = files.length;
    completedFiles = 0;
    chunkTotalProgressBar.value = 0;

    filesQueue = files;
    uploadNextFile();
  });

  pauseBtn.addEventListener("click", () => isPaused = true);
  resumeBtn.addEventListener("click", () => {
    if (isPaused) {
      isPaused = false;
      uploadChunks();
    }
  });

  function uploadNextFile() {
    if (filesQueue.length === 0) return;
    fileToUpload = filesQueue.shift();

    // ✅ Filter sesuai backend Go
    const allowedExt = [".jpg", ".png", ".pdf", ".mp4", ".iso", ".deb"];
    const ext = fileToUpload.name.substring(fileToUpload.name.lastIndexOf(".")).toLowerCase();
    if (!allowedExt.includes(ext)) {
      alert(`File ${fileToUpload.name} memiliki ekstensi tidak diizinkan.`);
      uploadNextFile();
      return;
    }

    const allowedMime = [
      "image/jpeg",
      "image/png",
      "application/pdf",
      "video/mp4",
      "application/x-iso9660-image",
      "application/vnd.debian.binary-package",
      "application/x-debian-package"
    ];
    if (!allowedMime.includes(fileToUpload.type)) {
      alert(`File ${fileToUpload.name} memiliki tipe tidak diizinkan (${fileToUpload.type}).`);
      uploadNextFile();
      return;
    }

    uploadId = `${fileToUpload.name}-${Date.now()}`;
    currentChunk = 0;
    uploadedChunks.clear();
    uploadChunks();
  }

  async function uploadChunks() {
    const token = localStorage.getItem("token");
    if (!token) return alert("Belum login!");

    const totalChunks = Math.ceil(fileToUpload.size / CHUNK_SIZE);
    chunkProgressBar.max = totalChunks;
    chunkProgressBar.value = 0;

    try {
      const resumeRes = await fetch(`/resume?upload_id=${uploadId}`, {
        headers: { "Authorization": "Bearer " + token }
      });
      const uploadedList = await resumeRes.json();
      uploadedChunks = new Set(uploadedList.map(c => c.toString()));
    } catch {
      console.warn("Gagal ambil status resume, lanjut dari awal.");
    }

    while (currentChunk < totalChunks) {
      if (isPaused) return;

      if (uploadedChunks.has(currentChunk.toString())) {
        currentChunk++;
        chunkProgressBar.value = currentChunk;
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
          headers: { "Authorization": "Bearer " + token },
          body: formData,
        });

        if (!res.ok) {
          alert(`Gagal upload chunk ${currentChunk}: ${res.status}`);
          return;
        }
      } catch (err) {
        console.error(`Chunk ${currentChunk} gagal diupload:`, err);
        return;
      }

      currentChunk++;
      chunkProgressBar.value = currentChunk;
    }

    try {
      const mergeRes = await fetch("/merge", {
        method: "POST",
        headers: {
          "Authorization": "Bearer " + token,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ uploadId, filename: fileToUpload.name }),
      });

      if (!mergeRes.ok) {
        alert("Gagal merge file!");
        return;
      }
    } catch (err) {
      alert("Gagal merge file!");
      return;
    }

    completedFiles++;
    chunkTotalProgressBar.value = (completedFiles / totalFiles) * 100;
    uploadNextFile();
  }

});
