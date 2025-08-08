document.addEventListener("DOMContentLoaded", () => {
    const token = localStorage.getItem("token");
    if (!token) {
        alert("Harap login terlebih dahulu!");
        window.location.href = "/login.html";
        return;
    }

    const fileList = document.getElementById("fileList");
    const dateFilter = document.getElementById("dateFilter");
    const limitSelect = document.getElementById("limitSelect");
    const prevBtn = document.getElementById("prevBtn");
    const nextBtn = document.getElementById("nextBtn");
    const pageInfo = document.getElementById("pageInfo");

    let page = 1;
    let totalPages = 1;

    async function loadFiles() {
        fileList.innerHTML = "<p>Memuat daftar file...</p>";

        const date = dateFilter.value;
        const limit = limitSelect.value;

        const res = await fetch(`/list-json?page=${page}&limit=${limit}&date=${date}&_=${Date.now()}`, {
            headers: { "Authorization": "Bearer " + token },
            cache: "no-store"
        });

        if (!res.ok) {
            fileList.innerHTML = "<p>Gagal memuat data.</p>";
            return;
        }

        const data = await res.json();
        const files = data.files || data;
        totalPages = data.totalPages || 1;

        if (!files.length) {
            fileList.innerHTML = "<p>Tidak ada file.</p>";
            return;
        }

        fileList.innerHTML = "";
        files.forEach(file => {
            const div = document.createElement("div");
            div.className = "file-item";
            div.innerHTML = `
                <span>${file.filename} - ${file.uploaded_at || ''}</span>
                <div>
                    <button class="downloadBtn" data-file="${file.filename}">Download</button>
                    <button class="deleteBtn" data-file="${file.filename}">Hapus</button>
                </div>
            `;
            fileList.appendChild(div);
        });

        pageInfo.textContent = `Halaman ${page} dari ${totalPages}`;

        // ✅ Event tombol download pakai fetch + blob
        document.querySelectorAll(".downloadBtn").forEach(btn => {
            btn.addEventListener("click", async (e) => {
                const filename = e.target.getAttribute("data-file");

                try {
                    const res = await fetch(`/download?file=${encodeURIComponent(filename)}&_=${Date.now()}`, {
                        headers: { "Authorization": "Bearer " + token }
                    });

                    if (!res.ok) {
                        alert("Gagal mengunduh file");
                        return;
                    }

                    const blob = await res.blob();
                    const url = window.URL.createObjectURL(blob);

                    const a = document.createElement("a");
                    a.href = url;
                    a.download = filename;
                    document.body.appendChild(a);
                    a.click();
                    a.remove();

                    window.URL.revokeObjectURL(url);
                } catch (err) {
                    alert("Terjadi kesalahan saat mengunduh file.");
                    console.error(err);
                }
            });
        });

        // Event untuk tombol hapus
        document.querySelectorAll(".deleteBtn").forEach(btn => {
            btn.addEventListener("click", async (e) => {
                const filename = e.target.getAttribute("data-file");
                if (!confirm(`Yakin mau hapus file ${filename}?`)) return;

                const delRes = await fetch(`/delete?file=${encodeURIComponent(filename)}&_=${Date.now()}`, {
                    method: "DELETE",
                    headers: { "Authorization": "Bearer " + token },
                    cache: "no-store"
                });

                if (delRes.ok) {
                    alert("File berhasil dihapus.");
                    loadFiles();
                } else {
                    alert("Gagal menghapus file.");
                }
            });
        });
    }

    // Event listeners filter & pagination
    dateFilter.addEventListener("change", () => { page = 1; loadFiles(); });
    limitSelect.addEventListener("change", () => { page = 1; loadFiles(); });

    prevBtn.addEventListener("click", () => {
        if (page > 1) {
            page--;
            loadFiles();
        }
    });

    nextBtn.addEventListener("click", () => {
        if (page < totalPages) {
            page++;
            loadFiles();
        }
    });

    // Load awal
    loadFiles();
});
