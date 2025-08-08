const form = document.getElementById('loginForm');
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const formData = new FormData(form);
      const response = await fetch('/login', {
        method: 'POST',
        body: formData
      });
      const token = await response.text();
      if (response.ok) {
        localStorage.setItem('token', token);
        alert('Login berhasil!');
        window.location.href = '/upload.html';
      } else {
        alert('Login gagal!');
      }
});