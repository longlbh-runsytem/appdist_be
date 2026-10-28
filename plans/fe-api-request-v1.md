# Kế hoạch triển khai — FE API request v1 (GMO App Delivery)

## Trạng thái: ĐÃ TRIỂN KHAI (2026-10-02)

Toàn bộ endpoint trong tài liệu FE đã có (trừ `POST /releases/:token/unlock` — FE ghi là tuỳ chọn).
Các điểm chặn B1–B7 ở mục 3 được làm theo **đề xuất** trong file này. Kiểm thử đầu-cuối 129 kịch
bản (quyền, scope, side effect, mã trạng thái) trên DB + MinIO tạm: qua hết.

### Khác với tài liệu FE — cần báo FE

| # | Tài liệu FE | Thực tế |
|---|---|---|
| 1 | Path gốc `/groups`, `/members`... | Prefix `/api/v1` (đặt vào baseURL) |
| 2 | Response trả thẳng object/mảng | Bọc `{code, message, data}` — đọc `res.data`. Lỗi: `{statusCode, message, errorKey}` |
| 3 | `PATCH /apps/:id/*`, `.../managing-groups`, `.../default-tester-groups` trả object rút gọn | Trả **toàn bộ** `AppDTO` (superset, không vỡ code FE) |
| 4 | `PATCH /builds/:id/archive`, `.../tester-groups` trả object rút gọn | Trả toàn bộ `BuildDTO` |
| 5 | `/members/*` vừa là tài khoản vừa là thành viên nhóm; `POST /members {email, role}` không có mật khẩu | **Tách 2 khái niệm** (quyết định của BE, 2026-10-02): tài khoản = `/users/*` — `POST /users/create {email, name, password, role}` dùng được ngay, `GET /users`, `/users/deleted`, `/users/{id}`, `PATCH /users/{id}/role`, `/status`, `DELETE`, `/restore`, thêm `GET /users/{id}/groups`. Thành viên nhóm = `POST/DELETE /groups/{id}/members/{userId}` (1 user nhiều nhóm). `UserDTO.invitedAt` → `createdAt`. `/members/*` đã bỏ |
| 6 | Tester "login (role tester)" | Thêm email vào tester group → tạo tài khoản role `tester` + `inviteUrl` trong response. `/tester/*` mở cho mọi tài khoản đăng nhập, kết quả lọc theo tester group của chính họ |
| 7 | `POST /builds/upload` nhận `platform`, `version`, `buildNumber` | `version`/`buildNumber` bị bỏ qua (đọc từ file); `platform` nếu gửi phải khớp file. Bundle id trong file phải khớp App, sai → 400 `ErrBundleIdMismatch` |
| 8 | `POST /quick-share` public | Public đúng như FE (từ 2026-10-09, migration 000009 cho `builds.uploaded_by` NULL với quick share). Có Bearer hợp lệ thì ghi lại người upload; Bearer sai → 401 |
| 9 | `isPublic` chưa định nghĩa | `false` = link `/d/:token` chỉ chạy khi kèm vé `?t=`; `GET /releases/:token` (kèm Bearer của người có quyền) trả `installUrl` có vé |
| 10 | `expiresAt`/`downloadLimit` của App | Áp cho mọi build của App; `downloadLimit` tính cho **mỗi** build. Hết → 410 |
| 11 | `POST /releases/:token/unlock` | Chưa làm; `requiresPassword` luôn `false` (FE đã ghi là chấp nhận được) |
| 12 | `GET /stats/downloads?from&to` | Định dạng `YYYY-MM-DD`, giờ Việt Nam, bỏ trống = 7 ngày gần nhất, có đủ ngày count = 0 |
| 13 | `GET /builds` không phân trang | `GET /builds` và `GET /apps/{id}/builds` (cùng params): `search` (version/buildNumber/changelog), `platform`, `status`, `from`/`to` (uploadedAt), `sort` = `uploadedAt`\|`version` (semver)\|`buildNumber`, `order`, `page`/`pageSize` — trả object phân trang như `GET /apps`, không có `counts`; `limit` đã bỏ. Không gồm quick share |
| 14 | — | `BuildDTO` có thêm `appName`, `bundleId`, `minOsVersion`, `metadata`, `downloadCount`, `downloadUrl`, `warnings`; `ReleaseDTO` thêm `pageUrl` |
| 15 | `parentId` của Group | Chỉ lưu để hiển thị cây, **không** kế thừa quyền |

### Việc còn lại

- Mật khẩu cho Release (B7) — cần thiết kế nếu làm.
- Gửi email lời mời tự động — hiện trả `inviteUrl` để người mời tự gửi.
- Trang `/upload` đã cập nhật theo API mới (chọn App / Chia sẻ nhanh) nhưng chưa kiểm thử trên trình duyệt.

---

Đối chiếu `api-request-v1.md` (FE) với BE hiện tại (`db/docs/db.dbml`, migrations 000001–000002,
các module `users`, `builds`, `downloads`).

---

## 1. Tóm tắt đánh giá

Tài liệu FE được viết dựa trên **mock**, không phải BE hiện tại. Vì vậy một phần "cần sửa" đã có sẵn,
nhưng phần lớn yêu cầu là **mô hình dữ liệu mới**:

| | BE hiện tại | FE yêu cầu |
|---|---|---|
| Đơn vị trung tâm | `build` độc lập (kiểu Diawi): upload → link | `App` sở hữu nhiều `Build`, theo platform |
| Phân quyền | `admin` / `member`, member chỉ thấy bản của mình | `owner` / `admin` / `member` + **scope theo Nhóm quản trị** |
| Người tải | ẩn danh, chỉ cần link | thêm **tester đăng nhập** (`/tester/my-apps`) |
| Trạng thái | `users.active bool`, `builds.active bool` | `status: active\|disabled\|deleted` (soft delete + restore) |
| Upload | presigned PUT thẳng lên B2, file không qua server | multipart qua server + API Key cho CI |

Ước lượng: **8 bảng mới, 2 bảng sửa, ~55 endpoint mới/sửa.** Phần users/members làm được ngay; phần
tester, approval, quick share cần chốt thiết kế trước (mục 3).

### Những gì FE nói "thiếu" nhưng BE đã có

- `POST /auth/login` đã dùng `email`, không có `username`.
- `UserDTO` đã có `role` (`admin|member`) và `email`; không có field `username`.
- `GET /users/me` đã trả `role`. Chỉ thiếu `status` và giá trị `owner`.
- Link tải `/d/:token`, đếm lượt tải (bảng `downloads`), đọc thông tin + icon từ file, QR — đã có.

---

## 2. Quy ước API cần chốt với FE (ảnh hưởng mọi endpoint)

Tài liệu FE khác BE ở 5 quy ước chung. Phải chốt **trước khi** viết endpoint đầu tiên, nếu không sẽ
sửa lại toàn bộ.

| # | Quy ước | BE hiện tại | FE mong đợi | Đề xuất |
|---|---|---|---|---|
| C1 | Prefix | `/api/v1/...` | `/groups`, `/members`... | **Giữ `/api/v1`**, FE đặt baseURL = `https://<domain>/api/v1`. |
| C2 | Vỏ response | `{code, message, data, paging}` | trả thẳng object/mảng | **Giữ vỏ `SuccessRes`** cho nhất quán với API đã có và lỗi `AppError`; FE đọc `res.data`. Danh sách phân trang: `data = {items, page, pageSize, total, totalPages, hasMore}`. |
| C3 | Tên field JSON | `snake_case` | `camelCase` (`parentId`, `iconUrl`...) | **Chuyển sang `camelCase`** cho toàn bộ `/api/v1`. Đổi bây giờ rẻ: consumer duy nhất hiện tại là trang `/upload` nội bộ. Kèm sửa JS trong `web/templates`. |
| C4 | HTTP status | luôn 200 | 201 khi tạo, 204 khi xoá | Làm theo FE: thêm `common.HandleCreated` (201) và `common.HandleNoContent` (204). |
| C5 | Kiểu ID | `uuid` (users), `bigint` (builds) | chuỗi | Bảng mới dùng `uuid`. `builds.id` giữ `bigint` trong DB, JSON trả dạng chuỗi (`json:"id,string"`). |

---

## 3. Vấn đề chặn — cần FE/product trả lời trước khi code

Xếp theo mức chặn. Mỗi mục có đề xuất để rút ngắn thảo luận.

### B1. Tester đăng nhập bằng gì? (chặn toàn bộ `/tester/*`)

- `GET /tester/my-apps` ghi "Quyền: login (role tester)", nhưng `role` của FE chỉ có
  `owner|admin|member`. Tester được thêm theo **email** vào tester group, không có mật khẩu.
- **Đề xuất:** thêm role `tester` vào `user_role`. Khi thêm email vào tester group mà chưa có tài khoản
  → tạo user `role=tester` + lời mời (B2). `my-apps` khớp theo `users.id ↔ testers.user_id`. Tester
  không truy cập được các API nội bộ (`RequireRole(owner, admin, member)`).

### B2. Mời member/tester thì đặt mật khẩu ở đâu?

- `POST /members` chỉ có `{email, role}`, không có mật khẩu, response có `invitedAt`. BE **chưa có
  hạ tầng gửi email**.
- **Đề xuất v1:** tạo user trạng thái chờ + bảng `invitations` (token băm, hạn 7 ngày). Response
  trả thêm `inviteUrl` để người mời tự gửi (Slack/chat). Thêm `POST /auth/accept-invite
  {token, password, name}`. Gửi email tự động để sau.

### B3. Upload multipart qua server vs presigned (chặn `POST /builds/upload`)

- BE hiện tại cố ý **không cho file đi qua server** (presigned PUT, 3 bước) vì file IPA/APK vài trăm MB.
- **Đề xuất:** làm cả hai.
  - Web FE: dùng luồng 3 bước có sẵn (`POST /builds/uploads` → PUT B2 → `POST /builds`), chỉ thêm `appId`.
  - CI: thêm `POST /builds/upload` multipart (đúng như FE mô tả), server **stream** part `file` thẳng
    lên B2 (không buffer cả file), rồi đi chung bước parse.

### B4. Client gửi `platform`, `version`, `buildNumber`

- FE: "BE tự đọc từ file nếu không truyền". Quy tắc dự án (AGENTS.md): **thông tin app luôn đọc từ
  file, không tin client**.
- **Đề xuất:** bỏ qua 3 field này (không đưa vào DTO). Thêm kiểm tra mới: `bundleId` trong file phải
  khớp `app.platforms[platform].bundleId`, sai → 400 `ErrBundleIdMismatch`. Ngăn upload nhầm app.

### B5. Quick Share công khai

- "Quyền: public" = bất kỳ ai cũng upload được file 1 GB vào bucket B2 (trả tiền theo dung lượng) —
  rủi ro lạm dụng cao.
- **Đề xuất:** bắt buộc login (mọi role nội bộ). Nếu buộc phải public: giới hạn dung lượng riêng
  (vd 200 MB) + rate limit theo IP.

### B6. Ngữ nghĩa các cờ của App

- `isPublic=false` nghĩa là gì? Đề xuất: chỉ tester trong tester group được gán **và đã đăng nhập**
  mới tải được; `true` = ai có link cũng tải được (như hiện tại).
- `expiresAt` / `downloadLimit` của App áp cho từng build hay tổng app? Đề xuất: mỗi build; hết hạn
  hoặc vượt limit → `/d/:token` trả 410.
- `parentId` của Group: member nhóm cha có quyền trên App của nhóm con không? Đề xuất v1: **lưu
  nhưng không kế thừa quyền**.
- Archive build → link `/d/:token` còn chạy không? Đề xuất: không (410).

### B7. Duyệt build, mật khẩu Release, cách tính Stats

Chính FE ghi "chưa có thiết kế". Đề xuất đưa ra khỏi đợt đầu (phase 7); `requiresPassword` luôn
`false`; `totalDownloads` = số dòng `downloads` (tải file thật, đã có), lọc theo scope như App.

---

## 4. Thay đổi DB

Theo AGENTS.md: **sửa `db/docs/db.dbml` trước**, rồi viết migration khớp theo; sqlc đọc `db/migrations`.

### 4.1. Enum

| Enum | Thay đổi |
|---|---|
| `user_role` | thêm `owner` (và `tester` nếu chốt B1). `ALTER TYPE ... ADD VALUE` không dùng được giá trị mới trong cùng transaction → tách migration riêng, chạy trước. |
| `entity_status` (mới) | `active`, `disabled`, `deleted` — dùng chung cho users, groups, testers. |
| `build_status` (mới) | `processing`, `ready`, `pending_approval`, `rejected`, `archived`. Parse hiện chạy đồng bộ nên build mới ra luôn `ready` / `pending_approval`; giữ `processing` cho khi chuyển sang xử lý nền. |
| `build_source` (mới) | `app`, `quick_share`. |

### 4.2. Bảng sửa

**`users`**
- `active bool` → `status entity_status NOT NULL DEFAULT 'active'`. Backfill `active=false → disabled`, rồi drop `active`.
- Thêm `invited_by uuid NULL` (FK users). `invitedAt` = `created_at` có sẵn.
- Data migration: admin tạo sớm nhất → `owner` (hệ thống phải có ít nhất 1 owner). Cập nhật `cmd/createadmin` → tạo `owner`.

**`builds`**
- Thêm `app_id uuid NULL` (FK apps; NULL cho quick share và bản cũ trước migration).
- Thêm `source build_source NOT NULL DEFAULT 'app'`.
- `active bool` → `status build_status NOT NULL DEFAULT 'ready'`. Backfill `active=false → archived`.
- Index `(app_id, created_at)`, `(status, expires_at)` (cho cron dọn quick share).
- Giữ nguyên `app_name`, `bundle_id`, `icon_key`... — là **ảnh chụp từ file** lúc upload, không phải
  thông tin của App.

### 4.3. Bảng mới

```
groups                 id uuid pk, name, parent_id uuid null → groups, status entity_status,
                       created_by → users, created_at, updated_at
group_members          group_id → groups, user_id → users, added_at   pk(group_id, user_id)

tester_groups          id uuid pk, name, created_by → users, created_at, updated_at
testers                id uuid pk, tester_group_id → tester_groups, email, user_id null → users,
                       status entity_status, created_at, updated_at
                       unique(tester_group_id, lower(email))

apps                   id uuid pk, name, ios_bundle_id null, android_bundle_id null,
                       icon_key null, is_public bool, expires_at null, download_limit int null,
                       requires_approval bool, auto_assign_tester_groups bool,
                       created_by → users, created_at, updated_at
                       check (ios_bundle_id is not null or android_bundle_id is not null)
                       unique partial index ios_bundle_id / android_bundle_id
app_managing_groups    app_id → apps, group_id → groups               pk(app_id, group_id)
app_default_tester_groups app_id → apps, tester_group_id → tester_groups   pk(...)
build_tester_groups    build_id → builds, tester_group_id → tester_groups  pk(...)

app_api_keys           id uuid pk, app_id → apps, label, token_hash (sha256, unique),
                       token_preview varchar(8), created_by → users, last_used_at null,
                       revoked_at null, created_at

invitations            id uuid pk, user_id → users, token_hash unique, expires_at,
                       accepted_at null, created_at                    (nếu chốt B2)

approval_requests      id uuid pk, build_id → builds unique, requested_by → users,
                       requested_at, decision null ('approve'|'reject'), note,
                       reviewed_by null → users, reviewed_at null      (phase 7, sau khi chốt B7)
```

Ghi chú thiết kế:
- `platforms` là 2 cột `ios_bundle_id` / `android_bundle_id` thay vì bảng con: chỉ có đúng 2 platform,
  query đơn giản hơn, unique index dễ.
- `apps.icon_key` cập nhật theo icon của build mới nhất → `Build.iconUrl` lấy từ App cha như FE yêu cầu.
- API key lưu **sha256**, không lưu plaintext; `tokenPreview` = 6 ký tự cuối.
- Không dùng `deleted_at`; soft delete bằng `status='deleted'` theo đúng mô hình FE.

### 4.4. Thứ tự migration

| File | Nội dung |
|---|---|
| `000003_role_owner` | `ALTER TYPE user_role ADD VALUE 'owner'` (+ `'tester'`) |
| `000004_user_status` | `entity_status`, `users.status`, `invited_by`, backfill, promote owner, drop `active` |
| `000005_group` | `groups`, `group_members` |
| `000006_tester_group` | `tester_groups`, `testers` |
| `000007_app` | `apps`, `app_managing_groups`, `app_default_tester_groups` |
| `000008_build_v2` | enums build, cột mới của `builds`, `build_tester_groups` |
| `000009_api_key` | `app_api_keys` |
| `000010_invitation` | `invitations` (sau khi chốt B2) |
| `000011_approval` | `approval_requests` (sau khi chốt B7) |

---

## 5. Scope — quy tắc phân quyền dùng chung

Gần như mọi endpoint App/Build cần câu hỏi "user này có quyền trên App X không". Gom vào **một chỗ**:

- `owner`: mọi App.
- `admin` / `member`: App có ít nhất 1 managing group mà user là thành viên **và** group `status='active'`.
- Ngoài scope → **404** (không phải 403), theo yêu cầu FE — không để lộ App tồn tại.

Hiện thực: 1 query sqlc `ListAppIDsInScope(user_id)` + 1 query `IsAppInScope(user_id, app_id)`, và
các query list nhận `sqlc.narg(scope_user_id)` (NULL = owner, không lọc). Service `apps` export
`RequireAppInScope(ctx, user, appID)` để module builds/tokens dùng lại.

Ma trận role (theo tài liệu FE):

| Hành động | owner | admin | member | tester |
|---|---|---|---|---|
| Xem members, groups, tester groups | ✓ | ✓ | ✓ | — |
| Tạo/khoá/xoá member | ✓ mọi role | ✓ chỉ `member`, không đụng owner | — | — |
| Đổi role member | ✓ | 403 | — | — |
| Quản lý group, app, API key | ✓ | ✓ (app: trong scope) | — | — |
| Upload build, gán tester group cho build | ✓ | ✓ | ✓ (trong scope) | — |
| Quản lý tester group/tester | ✓ | ✓ | ✓ | — |
| `/tester/my-apps` | — | — | — | ✓ |

Guard bổ sung: không tự khoá/xoá/đổi role chính mình (đã có `errCannotModifySelf`); không được hạ
quyền/khoá **owner cuối cùng**.

---

## 6. API + DTO theo module

Ký hiệu: 🆕 mới · ✏️ sửa · ♻️ thay thế API cũ. Mọi path dưới `/api/v1` trừ khi ghi khác.

### 6.1. users / members (`modules/users`)

| Endpoint | | Ghi chú |
|---|---|---|
| `POST /auth/login` | ✏️ | response `user` dùng `UserDTO` mới. Chặn login nếu `status != active`. |
| `GET /users/me` | ✏️ | như trên |
| `POST /users/me/password` | — | giữ |
| `POST /members` | ♻️ | thay `POST /users`. Body `{email, role}`. Admin chỉ tạo `member`. 201 |
| `GET /members` | ♻️ | thay `GET /users`. Loại `deleted`, không trả `tester` |
| `GET /members/deleted` | 🆕 | owner/admin |
| `GET /members/:id` | 🆕 | 404 nếu không có |
| `PATCH /members/:id/role` | ♻️ | tách từ `PATCH /users/:id`. Chỉ owner |
| `PATCH /members/:id/status` | ♻️ | `{status: active\|disabled}`. Khoá → `BlockAllUserSessions` (logic có sẵn) |
| `DELETE /members/:id` | 🆕 | soft delete + block sessions. 204 |
| `POST /members/:id/restore` | 🆕 | → `active` |
| `POST /auth/accept-invite` | 🆕 | nếu chốt B2 |

`RequireRole` hiện kiểm `user.Active` → đổi thành `user.Status == active`.

```go
// modules/users/dto
type MemberDTO struct {
    ID        uuid.UUID     `json:"id"`
    Name      string        `json:"name"`      // full_name; mới mời = email
    Email     string        `json:"email"`
    Role      sqlc.UserRole `json:"role"`      // owner|admin|member
    Status    sqlc.EntityStatus `json:"status"`
    InvitedAt time.Time     `json:"invitedAt"` // created_at
}
type MemberRefDTO struct { ID, Name, Email, Role }      // dùng trong Group.members
type CreateMemberPayload  struct { Email string `binding:"required,email"`; Role sqlc.UserRole `binding:"required,oneof=owner admin member"` }
type UpdateRolePayload    struct { Role   sqlc.UserRole     `binding:"required,oneof=owner admin member"` }
type UpdateStatusPayload  struct { Status sqlc.EntityStatus `binding:"required,oneof=active disabled"` }
```

`UserDTO` hiện tại đổi tên thành `MemberDTO` (bỏ `active`, `full_name` → `name`, thêm `status`, `invitedAt`).

### 6.2. groups (`modules/groups` — mới)

`POST /groups` · `GET /groups` · `GET /groups/deleted` · `GET /groups/:id` ·
`POST|DELETE /groups/:id/members/:memberId` · `PATCH /groups/:id/status` · `DELETE /groups/:id` ·
`POST /groups/:id/restore`

```go
type GroupDTO struct {
    ID       uuid.UUID         `json:"id"`
    Name     string            `json:"name"`
    ParentID *uuid.UUID        `json:"parentId"`
    Status   sqlc.EntityStatus `json:"status"`
    Members  []MemberRefDTO    `json:"members"`
}
type CreateGroupPayload struct { Name string `binding:"required,max=100"`; ParentID *uuid.UUID }
```

- `members` resolve bằng **1 query** `ListGroupMembers(group_ids[])` rồi ghép trong Go — tránh N+1 khi list.
- Thêm/bớt member là idempotent (`ON CONFLICT DO NOTHING`); thêm user `deleted`/`tester` → 400.
- `parentId` phải tồn tại, không trỏ vào chính nó; thao tác lên group `deleted` (trừ restore) → 404.
- Gin 1.12 ưu tiên route tĩnh `/groups/deleted` trước `/groups/:id` — không xung đột.

### 6.3. tester groups (`modules/testers` — mới)

`POST|GET /tester-groups` · `GET /tester-groups/:id` · `GET /tester-groups/:id/testers` ·
`GET /tester-groups/:id/testers/deleted` · `POST /tester-groups/:id/testers` (`{emails[]}`, tối đa 100,
dedupe, email đã có thì trả lại bản ghi cũ) · `PATCH .../testers/:testerId/status` ·
`DELETE .../testers/:testerId` · `POST .../testers/:testerId/restore`

```go
type TesterGroupDTO struct { ID uuid.UUID `json:"id"`; Name string `json:"name"`; MemberCount int32 `json:"memberCount"` } // đếm tester active
type TesterDTO      struct { ID uuid.UUID `json:"id"`; Email string `json:"email"`; Status sqlc.EntityStatus `json:"status"` }
type AddTestersPayload struct { Emails []string `json:"emails" binding:"required,min=1,max=100,dive,email"` }
```

### 6.4. apps (`modules/apps` — mới)

`POST /apps` · `GET /apps` (phân trang + `search`, `platform`, `from`, `to`) · `GET /apps/:id` ·
`PATCH /apps/:id/info` · `POST|DELETE /apps/:id/managing-groups/:groupId` ·
`PATCH /apps/:id/visibility` · `PATCH /apps/:id/approval` · `PATCH /apps/:id/auto-assign-tester-groups` ·
`POST|DELETE /apps/:id/default-tester-groups/:testerGroupId`

```go
type AppDTO struct {
    ID                     uuid.UUID      `json:"id"`
    Name                   string         `json:"name"`
    Platforms              PlatformsDTO   `json:"platforms"`   // {ios?:{bundleId}, android?:{bundleId}}
    IconURL                *string        `json:"iconUrl"`     // presigned từ apps.icon_key
    IsPublic               bool           `json:"isPublic"`
    ManagingGroups         []RefDTO       `json:"managingGroups"`      // {id, name}
    CreatedAt              time.Time      `json:"createdAt"`
    ExpiresAt              *time.Time     `json:"expiresAt"`
    DownloadLimit          *int32         `json:"downloadLimit"`
    RequiresApproval       bool           `json:"requiresApproval"`
    AutoAssignTesterGroups bool           `json:"autoAssignTesterGroups"`
    DefaultTesterGroups    []RefDTO       `json:"defaultTesterGroups"`
}
type CreateAppPayload struct {
    Name             string       `binding:"required,max=100"`
    Platforms        PlatformsDTO `binding:"required"`          // ít nhất 1 platform
    ManagingGroupIDs []uuid.UUID  `json:"managingGroupIds" binding:"required,min=1"`
}
type ListAppsQuery struct {
    Search   string     `form:"search"`
    Platform string     `form:"platform" binding:"omitempty,oneof=ios android"`
    From, To *time.Time `form:"from"/"to"`
    Page     int32      `form:"page"     binding:"omitempty,min=1"`
    PageSize int32      `form:"pageSize" binding:"omitempty,min=1,max=100"`
}
```

- **Các PATCH con trả toàn bộ `AppDTO`**, không trả object rút gọn như ví dụ FE: một hàm `toDTO`
  duy nhất, FE cập nhật cache bằng 1 object. (Báo FE — là superset nên không vỡ code FE.)
- Admin không thuộc group nào mà tạo App với `managingGroupIds` không chứa group của mình → tự mất
  scope. Đề xuất: admin chỉ được gán group mình là thành viên (owner thì tuỳ ý).
- Gỡ managing group cuối cùng → 400 (App không còn ai quản lý).
- `bundleId` trùng App khác → 409.

Dùng chung (đặt ở `common`):
```go
type Page[T any] struct {
    Items      []T   `json:"items"`
    Page       int32 `json:"page"`
    PageSize   int32 `json:"pageSize"`
    Total      int64 `json:"total"`
    TotalPages int32 `json:"totalPages"`
    HasMore    bool  `json:"hasMore"`
}
```
`total` lấy bằng `count(*) OVER()` trong cùng query list.

### 6.5. API keys (`modules/apps`, file riêng)

`POST /apps/:appId/tokens` (201, trả `token` plaintext **1 lần**) · `GET /apps/:appId/tokens` ·
`DELETE /apps/:appId/tokens/:tokenId` (204, set `revoked_at`)

- Token: `gmo_` + 32 ký tự ngẫu nhiên; lưu sha256.
- Middleware mới `middlewares.APIKeyOrBearer`: có `X-Api-Key` → tra hash, chưa revoke → gắn `appID`
  vào context, cập nhật `last_used_at` (không chặn request nếu ghi lỗi); không có → `RequiredAuth` như cũ.
- `createdBy` trả email (FE mong đợi chuỗi email).

### 6.6. builds (`modules/builds`)

| Endpoint | | Ghi chú |
|---|---|---|
| `POST /builds/uploads` | ✏️ | thêm `appId` (bắt buộc, kiểm scope) |
| `PUT /builds/uploads/content` | — | giữ (fallback CORS) |
| `POST /builds` | ✏️ | thêm `appId`, `changelog`; áp side effect bên dưới |
| `POST /builds/upload` | 🆕 | multipart cho CI (B3). `X-Api-Key` hoặc Bearer + `appId` |
| `GET /builds` | ✏️ | `?appId=`, lọc scope thay vì chỉ "của tôi"; phân trang `Page[T]` |
| `PATCH /builds/:id/archive` | 🆕 | owner/admin, → `archived` |
| `POST\|DELETE /builds/:id/tester-groups/:testerGroupId` | 🆕 | mọi role nội bộ, trong scope |

Bước tạo build (dùng chung cho `POST /builds` và `POST /builds/upload`), chạy trong `ExecTx`:
1. Parse file (có sẵn) → kiểm `platform` của App có cấu hình + `bundleId` khớp (B4).
2. Insert build: `app_id`, `source='app'`, `status = requiresApproval ? pending_approval : ready`.
3. `autoAssignTesterGroups` → copy `app_default_tester_groups` sang `build_tester_groups`.
4. `requiresApproval` → insert `approval_requests` (khi phase 7 xong; trước đó bỏ qua cờ này).
5. Cập nhật `apps.icon_key` = icon mới.

```go
type BuildDTO struct {
    ID           string           `json:"id"`           // bigint → chuỗi
    AppID        *uuid.UUID       `json:"appId"`
    Platform     sqlc.Platform    `json:"platform"`
    IconURL      *string          `json:"iconUrl"`      // từ App cha (quick share: icon riêng)
    Version      string           `json:"version"`
    BuildNumber  string           `json:"buildNumber"`
    Status       sqlc.BuildStatus `json:"status"`
    SizeBytes    int64            `json:"sizeBytes"`
    Changelog    string           `json:"changelog"`
    UploadedAt   time.Time        `json:"uploadedAt"`
    UploadedBy   string           `json:"uploadedBy"`   // email, JOIN users
    ExpiresAt    *time.Time       `json:"expiresAt"`
    TesterGroups []RefDTO         `json:"testerGroups"`
    ShareToken   string           `json:"shareToken"`   // builds.token có sẵn
}
```
Các field cũ (`download_url`, `metadata`, `warnings`, `min_os_version`...) giữ lại dạng camelCase —
FE bỏ qua field thừa được, trang `/upload` đang dùng `warnings`.

### 6.7. Tester (`modules/testers`)

`GET /tester/my-apps` (phân trang) · `GET /tester/my-apps/:appId/builds`

Điều kiện: `RequireRole(tester)`; tester `status=active`; build `status=ready` và có trong
`build_tester_groups` của một tester group chứa tester này. App ngoài phạm vi → 404.

### 6.8. Releases + Quick Share (`modules/downloads`)

| Endpoint | | Ghi chú |
|---|---|---|
| `GET /releases/:token` | 🆕 | bản JSON của trang `/d/:token` — tái dùng `DownloadService.Page`. `requiresPassword` luôn `false` |
| `POST /releases/:token/unlock` | ⏸ | hoãn (B7) |
| `POST /quick-share` | 🆕 | multipart; `source='quick_share'`, `app_id NULL`, `expires_at = now()+7d`. Quyền theo B5 |
| `/d/:token/*` | ✏️ | coi `status != ready` hoặc vượt `downloadLimit` là 410 |

```go
type ReleaseDTO struct {
    Token, AppName string; IconURL *string; Version, BuildNumber string; Platform sqlc.Platform
    SizeBytes int64; Changelog string; UploadedAt time.Time
    RequiresPassword bool; InstallURL string; ExpiresAt *time.Time
}
```
`installUrl`: iOS `itms-services://?action=download-manifest&url=<base>/d/<token>/manifest.plist`,
Android `<base>/d/<token>/app.apk` — logic này đã có trong `Page`, tách ra hàm dùng chung.

Cron dọn dẹp: goroutine `time.Ticker` 1 giờ/lần trong `main.go` (chưa cần hạ tầng job riêng): xoá
object B2 + bản ghi quick share hết hạn. Gộp luôn `DeleteExpiredSessions` (query đã có, chưa ai gọi).

### 6.9. Stats (`modules/stats` — mới)

`GET /stats/summary` · `GET /stats/downloads?from=&to=` (theo ngày, giờ Việt Nam, điền 0 cho ngày
không có lượt tải bằng `generate_series`). Cả hai lọc theo scope App của user (B7).

### 6.10. Approvals — phase 7, sau khi chốt B7

`GET /approvals` · `POST /approvals/:id/review {decision: approve|reject, note}` → build
`ready` / `rejected`.

---

## 7. Lộ trình

Mỗi phase kết thúc bằng: `make sqlc`, `make swagger`, test service, cập nhật trang `/upload` nếu bị ảnh hưởng.

| Phase | Nội dung | Phụ thuộc |
|---|---|---|
| **0** | Chốt C1–C5 và B1–B6 với FE/product | — |
| **1** | Nền: `camelCase`, `HandleCreated/NoContent`, `common.Page[T]`; migration 000003–000004; `/members/*`, login/me trả `status`; `createadmin` → owner | 0 |
| **2** | Groups (000005) | 1 |
| **3** | Tester groups + testers (000006) | 1 |
| **4** | Apps + scope + default tester groups (000007) | 2, 3 |
| **5** | Builds v2 (000008), API keys (000009), `POST /builds/upload` multipart, tester gán build | 4 |
| **6** | Releases JSON, quick share, cron, `/tester/my-apps`, invitations (000010) | 5, B1, B2 |
| **7** | Stats, approvals (000011), mật khẩu release | 6, B7 |

Phase 1 nên làm ngay vì FE xếp `UserDTO` là ưu tiên cao nhất và mọi phần khác phụ thuộc vào nó.

## 8. Thay đổi phá vỡ tương thích (báo FE + sửa trang `/upload`)

- Toàn bộ JSON chuyển `camelCase` (C3).
- `/users` (list/create/patch) → `/members/*`; `UserDTO.active` → `status`, `full_name` → `name`.
- `POST /builds/uploads` và `POST /builds` bắt buộc `appId` → trang `/upload` cần ô chọn App.
- `GET /builds`: `limit/offset` → `page/pageSize`, trả `Page[T]`.
- `builds.active` → `status`; link của build `archived` trả 410.
