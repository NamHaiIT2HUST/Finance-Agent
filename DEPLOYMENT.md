# Hướng dẫn Triển khai (Deployment) Bot Telegram lên Cloud

Bot quản lý chi tiêu của bạn đã được đóng gói bằng Docker. Dưới đây là hướng dẫn cơ bản để đưa bot lên các dịch vụ Cloud phổ biến (như Render hoặc Railway) để nó có thể chạy 24/7 mà không cần mở máy tính cá nhân.

## Điều kiện tiên quyết

1. **GitHub Account**: Cần có một tài khoản GitHub và đưa toàn bộ mã nguồn dự án lên một private repository.
2. **File Credentials**: Không nên push trực tiếp file `credentials.json` lên GitHub public. Nếu để private repo thì có thể push lên, nhưng cách chuẩn nhất là biến file JSON thành chuỗi Base64 và đưa vào biến môi trường (Environment Variable), sau đó decode lại trong code, hoặc mount volume. Tuy nhiên, ở phiên bản đơn giản hiện tại, nếu repo của bạn là **Private**, bạn có thể commit luôn `credentials.json` để Dockerfile copy vào.

---

## Tùy chọn 1: Render.com (Khuyên dùng vì dễ nhất)

Render là dịch vụ Cloud rất thân thiện cho Docker.

1. Truy cập [Render.com](https://render.com/) và đăng nhập bằng GitHub.
2. Ở Dashboard, chọn **New** -> **Web Service** (hoặc Background Worker nếu bạn không muốn expose port HTTP nào, nhưng Web Service là phổ biến nhất).
3. Kết nối với Repository chứa mã nguồn bot của bạn.
4. Trong phần cài đặt của dịch vụ:
   - **Name**: `finance-agent-bot`
   - **Environment**: Chọn `Docker` (Render sẽ tự động đọc `Dockerfile` trong repo của bạn).
   - **Region**: Chọn region gần bạn (ví dụ Singapore).
   - **Plan**: Chọn `Free` (Lưu ý: Plan Free của web service có thể sleep nếu không có web request, nhưng bot telegram long-polling vẫn giữ kết nối. Tốt nhất là dùng **Background Worker** thay vì Web Service).
5. **Cấu hình Biến môi trường (Environment Variables)**:
   Mở phần "Advanced" -> "Add Environment Variable" và nhập các biến có trong file `.env` của bạn:
   - `TELEGRAM_BOT_TOKEN`: `...`
   - `GEMINI_API_KEY`: `...`
   - `SPREADSHEET_ID`: `...`
   - `CHAT_ID`: `...`
6. Nhấp **Create Background Worker** / **Create Web Service**.
7. Chờ vài phút để Render build Docker Image và khởi chạy bot. Khi log báo `🤖 Đã đăng nhập thành công vào bot...`, bot của bạn đã online!

---

## Tùy chọn 2: Railway.app

Railway cung cấp trải nghiệm Developer cực tốt nhưng hiện đã bỏ gói Free vĩnh viễn (có trial).

1. Truy cập [Railway.app](https://railway.app/).
2. Chọn **New Project** -> **Deploy from GitHub repo**.
3. Chọn repo của dự án. Railway sẽ tự động nhận diện `Dockerfile`.
4. Sau khi project được tạo, click vào Service vừa tạo -> chuyển sang tab **Variables**.
5. Thêm tất cả các biến môi trường cần thiết (`TELEGRAM_BOT_TOKEN`, `GEMINI_API_KEY`, v.v.).
6. Railway sẽ tự động rebuild lại. Khi có tick xanh lá cây là bot đã chạy thành công.

---

## Tùy chọn 3: VPS Cá nhân (Dành cho tự quản lý)

Nếu bạn có VPS (Ubuntu/Debian) mua từ DigitalOcean, Vultr hoặc các nhà cung cấp VN:

1. SSH vào VPS của bạn.
2. Cài đặt Docker: `curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh`
3. Clone repo mã nguồn về VPS: `git clone https://github.com/your-username/Finance-Agent.git`
4. CD vào thư mục dự án: `cd Finance-Agent`
5. Tạo file `.env` trực tiếp trên VPS và điền các Key.
6. Build Docker image: 
   ```bash
   docker build -t finance-bot .
   ```
7. Chạy bot ở chế độ ngầm (background) với file `.env`:
   ```bash
   docker run -d --name finance_agent_container --env-file .env finance-bot
   ```
8. (Tùy chọn) Để bot tự khởi động lại khi VPS bị restart:
   ```bash
   docker update --restart unless-stopped finance_agent_container
   ```

## Xử lý sự cố (Troubleshooting)

- **Lỗi không đọc được credentials.json**: Đảm bảo file `credentials.json` nằm cùng cấp với binary khi chạy, Dockerfile hiện tại đã có lệnh `COPY credentials.json .`.
- **Lỗi múi giờ Cron job chạy sai**: Dockerfile đã cấu hình `ENV TZ=Asia/Ho_Chi_Minh` giúp Cron job chạy theo giờ VN.

Chúc bạn triển khai thành công!
