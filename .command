1.1ogin & Dapatkan Token
    curl -X POST -d "username=user1&password=pass1" http://localhost:8080/login
    Output:
    Token JWT, contoh:
    eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJVc2VybmFtZSI6InVzZXIxIiwiZXhwIjoxNjkxMDU3ODI4fQ.p5...

2.Upload File Pakai Token
    curl -F "file=@namafile.txt" \
    -H "Authorization: Bearer <TOKEN_KAMU>" \
    http://localhost:8080/upload

    Ganti:
    namafile.txt → nama file lokal kamu
    <TOKEN_KAMU> → token dari hasil login

    Contoh:
    curl -F "file=@tes.txt" \
    -H "Authorization: Bearer eyJhbGciOi..." \
    http://localhost:8080/upload

3.Download File Pakai Token
    curl -H "Authorization: Bearer <TOKEN_KAMU>" \
    "http://localhost:8080/download?file=namafile.txt" -O

    Contoh:
    curl -H "Authorization: Bearer eyJhbGciOi..." \
    "http://localhost:8080/download?file=tes.txt" -O

4.Jika Akses dari HP / Jaringan Lain
    Ganti localhost dengan IP lokal komputer/server kamu, misalnya:
    http://192.168.1.10:8080
    Gunakan:
    ip a | grep inet

