# Deploy AppDist BE lên server

**Clone về rồi chạy ngay là chưa được.** Repo không chứa secret, và `docker-compose.yml` không tự build image.
Trên server cần chuẩn bị 4 thứ:

1. File `prod.env` (secret thật). Tạo từ `prod.env.example`.
2. Image `appdist-be:<tag>`. Build ngay trên server, hoặc build ở máy dev rồi đẩy qua SSH.
3. Nginx + HTTPS (Let's Encrypt). **iOS chỉ cài app qua HTTPS có chứng chỉ hợp lệ.**
4. Tài khoản owner đầu tiên, tạo bằng `createadmin`.

Mỗi lần `docker compose up`, hệ thống chạy theo thứ tự: postgres sẵn sàng → `migrate` chạy migration mới → `api` khởi động.
Migration chạy tự động, không cần làm tay.

---

## 0. Yêu cầu

| Thứ | Ghi chú |
|---|---|
| Server Linux x86_64 | RAM ≥ 1 GB. File upload được stream thẳng lên B2, không lưu trên đĩa server |
| Docker Engine + plugin `docker compose` | https://docs.docker.com/engine/install/ |
| Nginx + certbot | Reverse proxy HTTPS |
| Domain trỏ về IP server | VD `api.dist.example.com` cho BE. FE là project riêng |
| Bucket Backblaze B2 + Application Key | Xem README mục "Cấu hình Backblaze B2". Production nên dùng bucket riêng |

Firewall chỉ mở 22, 80 và 443. **Không** mở 8080 hay 5432: API chỉ bind `127.0.0.1`, còn Postgres không publish cổng.

```bash
sudo ufw allow OpenSSH && sudo ufw allow 'Nginx Full' && sudo ufw enable
```

## 1. Clone code

Repo private thì nên dùng **deploy key** (chỉ đọc) riêng cho server, đừng chép SSH key cá nhân lên server:

```bash
ssh-keygen -t ed25519 -C "appdist-server" -f ~/.ssh/appdist_deploy -N ""
cat ~/.ssh/appdist_deploy.pub
```

Dán key vào GitHub: repo → Settings → Deploy keys → Add deploy key. **Không** tick "Allow write access".

```bash
cat >> ~/.ssh/config <<'EOF'
Host github-appdist
  HostName github.com
  User git
  IdentityFile ~/.ssh/appdist_deploy
  IdentitiesOnly yes
EOF

sudo mkdir -p /opt/appdist && sudo chown $USER /opt/appdist
git clone git@github-appdist:longlbh-runsytem/appdist_be.git /opt/appdist
cd /opt/appdist
```

## 2. Tạo `prod.env`

```bash
cp prod.env.example prod.env
chmod 600 prod.env
```

Các biến bắt buộc phải điền:

| Biến | Cách lấy / lưu ý |
|---|---|
| `POSTGRES_PASSWORD` | `openssl rand -hex 24`. Dùng hex để không phải URL-encode |
| `DB_SOURCE` | Thay `<POSTGRES_PASSWORD>` bằng đúng mật khẩu trên. Host giữ là `postgres` (tên service trong compose) |
| `TOKEN_SYMMETRIC_KEY` | **Đúng 32 ký tự**: `openssl rand -hex 16`. Đổi key thì mọi phiên đăng nhập mất hiệu lực |
| `DOMAIN` | URL HTTPS của BE, vd `https://api.dist.example.com`. Không có `/` ở cuối. Dùng để tạo link cài iOS, manifest, QR |
| `FRONTEND_URL` | URL web FE. Link tải `<FRONTEND_URL>/d/<token>` và link mời trỏ về đây |
| `CORS_ALLOWED_ORIGINS` | Origin của web FE, phân tách bằng dấu phẩy. **Không** dùng `*` |
| `B2_BUCKET`, `B2_KEY_ID`, `B2_APPLICATION_KEY` | Lấy từ Backblaze. `B2_ENDPOINT` không có `https://` |
| `SWAGGER_ENABLED` | Production để `false` |

⚠️ **Đặt mật khẩu DB cẩn thận ngay từ đầu.** Postgres chỉ đọc `POSTGRES_USER/PASSWORD/DB` ở **lần khởi tạo volume đầu tiên**.
Nếu sau đó sửa `POSTGRES_PASSWORD` trong file, mật khẩu trong DB không đổi, và `migrate`/`api` sẽ báo lỗi xác thực.
Muốn đổi thì chạy `ALTER USER appdist PASSWORD '...'` trong psql, hoặc xoá volume (mất dữ liệu).

## 3. Có image `appdist-be`

`docker-compose.yml` chỉ khai báo `image: appdist-be:${TAG}` mà không có `build:`. Thiếu image thì `up` sẽ báo lỗi
`pull access denied`. Bạn chọn một trong hai cách:

**Cách A: build ngay trên server** (đơn giản nhất, vì code đã clone sẵn ở đó)

```bash
docker build -t appdist-be:1.0.0 .
```

**Cách B: build trên máy Mac rồi đẩy qua SSH** (server yếu, không muốn build trên đó)

```bash
# chạy trên máy dev
make docker_build version=1.0.0
make docker_push_ssh version=1.0.0 host=user@server
```

Cách B vẫn cần clone repo trên server, vì compose mount `db/migrations` từ thư mục repo.

## 4. Chạy

```bash
cd /opt/appdist
echo "TAG=1.0.0" > .env      # compose tự đọc .env để thay ${TAG}; file này đã nằm trong .gitignore
docker compose up -d
docker compose ps            # api phải lên "healthy", migrate "exited (0)"
docker compose logs -f api
curl -s http://127.0.0.1:8080/health   # {"status":"ok"}
```

Lúc khởi động, server kiểm tra kết nối DB và B2. Sai key hoặc tên bucket thì `api` sẽ dừng và in lỗi rõ ràng trong log.

## 5. Tạo tài khoản owner đầu tiên

Hệ thống không có đăng ký công khai. Chạy binary `createadmin` có sẵn trong image:

```bash
docker compose run --rm --no-deps --entrypoint /app/createadmin api \
  -email owner@congty.vn -name "Owner" -password 'mat-khau-manh-8-72-ky-tu'
```

Máy dev/staging có thể thay bước này bằng `SEED_DEMO_DATA=true` trong env. Khi đó service `seed` tự tạo
tài khoản demo (`owner@example.com` / `owner123`, admin, member, tester) lúc DB còn trống.
Production **luôn** để `false`, vì mật khẩu demo rất yếu.

Lệnh này chỉ cần chạy một lần. Sau đó owner tạo các tài khoản khác qua web hoặc API.
Lưu ý mật khẩu sẽ nằm trong shell history. Sau khi tạo, xoá history (`history -d`) hoặc đổi mật khẩu trên web.

## 6. Nginx + HTTPS

`/etc/nginx/sites-available/appdist`:

```nginx
# Rate limit cho /api/v1/quick-share: endpoint public, ai cũng upload được.
limit_req_zone $binary_remote_addr zone=quickshare:10m rate=5r/m;

server {
    listen 80;
    server_name api.dist.example.com;

    # Upload IPA/APK lớn: phải >= MAX_UPLOAD_SIZE_MB
    client_max_body_size 1100m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Stream file thẳng vào BE, không buffer ra đĩa của Nginx
        proxy_request_buffering off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }

    location = /api/v1/quick-share {
        limit_req zone=quickshare burst=3 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_request_buffering off;
        proxy_read_timeout 600s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/appdist /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d api.dist.example.com     # tự thêm khối listen 443 + redirect HTTP→HTTPS
curl -s https://api.dist.example.com/health
```

Đừng quên bật CORS cho bucket B2 nếu web FE upload thẳng lên B2 (xem README).

## 7. Cập nhật phiên bản mới

```bash
cd /opt/appdist
git pull
docker build -t appdist-be:1.0.1 .        # hoặc dùng cách B ở bước 3
echo "TAG=1.0.1" > .env
docker compose up -d                      # migration mới tự chạy trước khi api khởi động
docker image prune -f                     # dọn image cũ (nên giữ lại 1 bản để rollback)
```

**Rollback:** chạy `echo "TAG=1.0.0" > .env && docker compose up -d`. Migration **không** tự chạy `down`.
Nếu bản mới có migration làm đổi schema, phải tự đánh giá xem bản cũ còn chạy được trên schema mới không.
Vì vậy nên backup DB trước mỗi lần cập nhật có migration.

## 8. Backup

Dữ liệu DB nằm trong volume `pgdata`. File cài đặt nằm trên B2, không ở server.

```bash
# backup
docker compose exec -T postgres pg_dump -U appdist appdist | gzip > /opt/backup/appdist-$(date +%F).sql.gz

# restore (vào DB trống)
gunzip -c appdist-2026-10-28.sql.gz | docker compose exec -T postgres psql -U appdist appdist
```

Nên đặt cron chạy hằng ngày và chép bản backup ra ngoài server.

⚠️ `docker compose down -v` **xoá volume, tức là xoá toàn bộ DB**. Dừng bình thường thì chỉ dùng `docker compose down`.

## Checklist trước khi giao cho người dùng

- [ ] `prod.env` có `chmod 600` và secret khác hẳn môi trường dev/test
- [ ] `TOKEN_SYMMETRIC_KEY` đúng 32 ký tự
- [ ] `SWAGGER_ENABLED=false`, `CORS_ALLOWED_ORIGINS` không phải `*`
- [ ] `https://<domain>/health` trả `{"status":"ok"}` với chứng chỉ hợp lệ
- [ ] Upload một IPA, mở link trên iPhone thật và cài được
- [ ] Đã tạo owner, đăng nhập được từ web FE
- [ ] Cổng 8080/5432 không truy cập được từ internet
- [ ] Đã có cron backup DB
