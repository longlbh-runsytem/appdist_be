# AppDist BE

Nơi lưu file cài đặt (IPA/APK) và tạo link tải cho khách — thay thế Diawi.

- Người upload cần tài khoản. Người tải chỉ cần link.
- IPA phải được ký sẵn (in-house) từ lúc build; server không ký lại.
- File lưu trên Backblaze B2 (bucket private). Mọi thông tin app đọc từ **bên trong** file.

Stack: Go, gin, PostgreSQL (pgx + sqlc), golang-migrate, PASETO, minio-go (B2).

## Chạy local

```bash
cp app.env.example app.env   # điền TOKEN_SYMMETRIC_KEY và các biến B2_*
make postgres && make createdb
make migrateup
make create_admin email=owner@example.com name="Owner" password=changeme123   # tạo owner đầu tiên
make server
```

### Dev không cần B2: dùng MinIO local

```bash
docker run -d --name appdist-minio -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
  --entrypoint sh quay.io/minio/minio -c 'mkdir -p /data/appdist-dev && minio server /data --console-address :9001'
```

```env
B2_ENDPOINT=http://localhost:9000   # tiền tố http:// chỉ dành cho dev
B2_REGION=us-east-1
B2_BUCKET=appdist-dev
B2_KEY_ID=minioadmin
B2_APPLICATION_KEY=minioadmin
```

Xem file đã upload tại http://localhost:9001.

## Cấu hình Backblaze B2

1. **Tạo bucket** — B2 Cloud Storage → Buckets → Create a Bucket
   - Files in Bucket: **Private** (link tải luôn đi qua server để đếm lượt và thu hồi được).
   - Region: chọn **EU Central** nếu được (gần Việt Nam hơn US).
   - Sau khi tạo, xem mục **Endpoint** của bucket, vd `s3.eu-central-003.backblazeb2.com`.
2. **Tạo Application Key** — Application Keys → Add a New Application Key
   - Allow access to Bucket(s): **chỉ bucket vừa tạo** (không dùng master key).
   - Type of Access: **Read and Write**.
   - Lưu lại `keyID` và `applicationKey` — applicationKey chỉ hiện **một lần**.
3. **Điền `app.env`**

   ```env
   B2_ENDPOINT=s3.eu-central-003.backblazeb2.com   # không có https://
   B2_REGION=eu-central-003                         # phần giữa endpoint
   B2_BUCKET=<tên bucket>
   B2_KEY_ID=<keyID>
   B2_APPLICATION_KEY=<applicationKey>
   ```

4. **CORS** — chỉ cần nếu upload từ **trình duyệt** (web admin). Bucket Settings → CORS Rules →
   Custom rules, cho phép origin của web admin với method `PUT` qua S3 API. Upload bằng
   script/CI (curl) thì bỏ qua bước này.

Server kiểm tra kết nối bucket ngay lúc khởi động — sai key/bucket sẽ dừng với lỗi rõ ràng.

## Giao diện web

Repo này chỉ là API. Giao diện (upload, quản lý, trang tải cho khách) nằm ở project FE riêng, gọi
`/api/v1`. Link đưa cho khách là trang FE `<FRONTEND_URL>/d/<token>`; trang đó lấy dữ liệu từ
`GET /api/v1/releases/:token` (chuyển tiếp nguyên `?t=` nếu link có).

Web FE nên PUT file **thẳng lên B2** trước, bucket chưa bật CORS thì gửi **qua server**
(`PUT /api/v1/builds/uploads/content`). Muốn upload thẳng lên B2 (nhanh hơn, không tốn băng
thông server) thì bật CORS cho S3 API bằng B2 CLI:

```bash
b2 bucket update appdist allPrivate --cors-rules '[{
  "corsRuleName": "appdist-upload",
  "allowedOrigins": ["https://<FRONTEND_URL>", "http://localhost:5173"],
  "allowedOperations": ["s3_put", "s3_get", "s3_head"],
  "allowedHeaders": ["*"],
  "exposeHeaders": ["ETag"],
  "maxAgeSeconds": 3600
}]'
```

## Mô hình

- **User** = tài khoản nội bộ (`/users`), owner/admin tạo kèm mật khẩu qua `POST /users/create`, dùng được ngay. Role toàn hệ
  thống: `owner` (toàn quyền) · `admin` (quản lý tài khoản role member/tester, nhóm, App) · `member` (upload
  build, quản lý tester) · `tester` (chỉ xem build được mời qua `/tester/*`, không vào API quản trị,
  không thuộc nhóm quản trị — hạ role xuống tester sẽ tự gỡ khỏi mọi nhóm).
- **Member** = thành viên của **nhóm quản trị** (`/groups/{id}/members/{userId}`). 1 user là thành viên
  được nhiều nhóm; thuộc nhóm nào thì quản lý App mà nhóm đó phụ trách. owner thấy mọi App; App ngoài
  phạm vi trả **404**. Khoá nhóm → thành viên mất quyền ngay. Bỏ khỏi nhóm không đụng tới tài khoản.
- **App** (`/apps`): gom build theo bundle id iOS/Android. Build upload lên phải đúng bundle id đã cấu hình.
- **Tester group / tester** (`/tester-groups`): người ngoài, đăng nhập bằng tài khoản role `tester`
  (tạo khi thêm email, kèm link mời) và chỉ thấy build `ready` được gán cho nhóm mình (`/tester/my-apps`).
- Xoá là **xoá mềm** (`status = deleted`), khôi phục bằng `.../restore`.

Quy ước API: prefix `/api/v1`, JSON `camelCase`, response bọc `{code, message, data}`, tạo trả 201,
xoá trả 204, lỗi trả `{statusCode, message, errorKey}`. Danh sách phân trang:
`data = {items, page, pageSize, total, totalPages, hasMore}`. Tài liệu đầy đủ: Swagger (`/swagger/index.html`).

## Upload build

### Từ CI/CD bằng API key

Tạo key: `POST /api/v1/apps/{appId}/tokens` (owner/admin) — `token` chỉ hiện **một lần**.

```bash
curl -X POST $API/api/v1/builds/upload \
  -H "X-Api-Key: $APPDIST_API_KEY" \
  -F "file=@build/app-release.apk" \
  -F "changelog=Sửa lỗi đăng nhập"
```

App lấy từ key. `version`/`buildNumber` luôn đọc từ file; `platform` nếu gửi phải khớp file.
File được stream thẳng lên storage, không buffer trên server.

### Từ web — 3 bước, file không đi qua server

```bash
# 1. Xin URL upload
curl -X POST $API/api/v1/builds/uploads -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"fileName":"Pharmago.ipa","fileSize":123456789}'
# → { uploadKey, uploadUrl, method: "PUT", expiresAt }

# 2. PUT file thẳng lên B2
curl -X PUT --upload-file Pharmago.ipa "$UPLOAD_URL"

# 3. Tạo build — server tự đọc bundle id, version, build number, icon...
curl -X POST $API/api/v1/builds -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"uploadKey":"builds/<user-id>/<uuid>.ipa","appId":"<app-id>","changelog":"Sửa lỗi đăng nhập"}'
# → BuildDTO { status, version, buildNumber, shareToken, downloadUrl, iconUrl, testerGroups, warnings, ... }
```

File không hợp lệ (không phải IPA/APK, IPA chưa ký, ký cho App Store, profile hết hạn, bundle id
không khớp App) bị từ chối và tự xoá khỏi storage.

Khi tạo build: App bật `autoAssignTesterGroups` → build kế thừa tester group mặc định; App bật
`requiresApproval` → build ở `pending_approval` cho tới khi owner/admin duyệt ở `/approvals`.

### Chia sẻ nhanh

`POST /api/v1/quick-share` (multipart, **không cần đăng nhập**; có Bearer hợp lệ thì ghi lại người
upload): không gắn App, link tự hết hạn sau 7 ngày; server dọn file hết hạn mỗi giờ.

⚠️ Endpoint public: ai cũng upload được file tới `MAX_UPLOAD_SIZE_MB` lên bucket. Khi đưa ra internet
nên đặt rate limit theo IP ở reverse proxy (nginx/Cloudflare) cho riêng route này.

## Schema

- Nguồn sự thật: `db/docs/db.dbml`. Migration: `db/migrations`. Sau khi sửa query: `make sqlc`.
- Migration `000008` gom build cũ thành App theo bundle id (để **public** cho link cũ vẫn chạy,
  chưa gán nhóm quản trị nên chỉ owner thấy). Migration `000004` nâng admin tạo sớm nhất thành owner.

## API

Xem Swagger cho đầy đủ body/response. Tóm tắt quyền:

| Nhóm | Endpoint | Quyền |
|---|---|---|
| Auth | `POST /auth/login` · `/refresh` · `/logout` · `/accept-invite` | public |
| Tài khoản của mình | `GET /users/me` · `POST /users/me/password` | đăng nhập (kể cả tester) |
| User (tài khoản) | `GET /users` · `/users/:id` · `/users/:id/groups` | nội bộ |
| | `POST /users/create` · `PATCH /users/:id/status` · `DELETE /users/:id` · `/restore` · `GET /users/deleted` | owner/admin (admin không đụng owner, chỉ tạo role member/tester) |
| | `PATCH /users/:id/role` | owner |
| Nhóm | `GET /groups` · `/groups/:id` | nội bộ |
| | `POST/DELETE /groups/:id/members/:userId` (thêm/bỏ thành viên) và còn lại dưới `/groups` | owner/admin |
| App | `GET /apps` · `/apps/:id` | nội bộ, trong scope |
| | còn lại dưới `/apps` (kể cả `/tokens`) | owner/admin, trong scope |
| Build | `POST /builds/upload` | `X-Api-Key` hoặc Bearer nội bộ |
| | `GET /builds` · upload 3 bước · `/builds/:id/tester-groups/*` | nội bộ, trong scope |
| | `PATCH /builds/:id/archive` · `/approvals/*` | owner/admin, trong scope |
| Tester | `/tester-groups/*` | nội bộ |
| | `/tester/my-apps/*` | đăng nhập |
| Release | `GET /releases/:token` | public (App không public: cần vé `?t=` hoặc quyền, trả link kèm vé) |
| | `POST /quick-share` | public (Bearer tuỳ chọn) |
| Thống kê | `/stats/summary` · `/stats/downloads` | nội bộ, trong scope |

### URL thiết bị của khách gọi thẳng (public, dưới `PUBLIC_BASE_URL`)

Trang tải HTML do FE phục vụ tại `<FRONTEND_URL>/d/:token`. BE chỉ có:

| Path | Nội dung |
|---|---|
| `/d/:token/manifest.plist` | Manifest cho `itms-services://` (chỉ bản iOS) |
| `/d/:token/app.ipa`, `/app.apk` | Ghi lượt tải rồi redirect sang link tạm trên storage (15 phút) |
| `/d/:token/icon.png`, `/qr.png` | Icon app, mã QR trỏ tới trang tải bên FE |

Mở trang tải không tính lượt tải. Chỉ tính request GET tải từ đầu file (HEAD và tải tiếp từ giữa file
không tính). Build chưa duyệt / bị từ chối / đã lưu trữ, hết hạn (build hoặc App) hoặc hết
`downloadLimit` trả 410.

App **không public**: mọi route `/d/:token/*` cần vé `?t=` (HMAC, hạn 24 giờ) — vé có sẵn trong
`installUrl`/`pageUrl` do `GET /api/v1/releases/:token` trả cho người có quyền, và trong `downloadUrl`
của build. iOS tải manifest/IPA không mang token đăng nhập nên quyền phải nằm trong URL. Trang FE
mở bằng link có vé thì gửi `GET /releases/:token?t=<vé>` — không cần đăng nhập, vé không được gia hạn.

⚠️ iOS chỉ cài qua `itms-services://` khi `PUBLIC_BASE_URL` là **HTTPS có chứng chỉ hợp lệ** —
test trên iPhone thật cần domain thật hoặc tunnel (vd `cloudflared tunnel --url http://localhost:8080`).
