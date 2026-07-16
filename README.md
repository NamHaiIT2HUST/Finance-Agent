# SaaS Finance AI - Ứng dụng Quản lý Tài chính Cá nhân Thông minh 💰🤖

**SaaS Finance AI** là một ứng dụng Web (Web Application) quản lý tài chính cá nhân tự động hóa bằng Trí tuệ Nhân tạo (AI). Khác với các ứng dụng ghi sổ truyền thống phải nhập tay từng con số, người dùng chỉ cần **"chat"** tự nhiên hoặc **gửi ảnh hóa đơn/bill chuyển khoản**, "bộ não" AI của hệ thống sẽ tự động bóc tách số liệu, phân loại danh mục và cập nhật biểu đồ theo thời gian thực.

Dự án được thiết kế theo kiến trúc **chịu tải cao (High Concurrency)** để sẵn sàng phục vụ hàng ngàn người dùng cùng lúc với tư cách là một phần mềm dịch vụ (SaaS).

---

## 🛠 Công nghệ cốt lõi (Tech Stack)
- **Frontend (Giao diện)**: HTML5, CSS3 (Modern Dark Theme, Glassmorphism, Responsive), Vanilla JavaScript.
- **Backend (Máy chủ)**: Golang (Goroutines, Channels) đảm bảo tốc độ xử lý siêu tốc và chịu tải lớn.
- **Cơ sở dữ liệu**: SQLite / GORM (Lưu trữ User, Giao dịch và Lịch sử Chat an toàn).
- **Trí tuệ nhân tạo (AI)**: Google Gemini 3.5 Flash (Xử lý ngôn ngữ tự nhiên và phân tích hình ảnh đa phương thức - Multimodal).
- **Bảo mật**: JWT (JSON Web Tokens) Authentication, Mã hóa mật khẩu Bcrypt.

---

## ✨ Các tính năng nổi bật (Version 2.0 - SaaS Edition)

### 1. Dành cho Người dùng (End User)
- **Tương tác Tự nhiên (Chat & Upload)**: Trò chuyện với AI để ghi nhận thu/chi, hoặc đơn giản là ném cho hệ thống 1 tấm ảnh hóa đơn siêu thị/chuyển khoản.
- **Tối ưu Hóa Hình ảnh (Client-Side Compression)**: Ảnh chụp từ điện thoại (dung lượng lớn 5-10MB) được nén siêu tốc tự động bằng Canvas API ngay trên trình duyệt trước khi gửi, giúp tiết kiệm băng thông và tăng tốc xử lý AI gấp 3 lần.
- **Bảng điều khiển (Dashboard) Real-time**: Thống kê Tổng thu, Tổng chi, Số dư và Biểu đồ phân bổ chi tiêu (Pie Chart) trực quan, được cập nhật ngay lập tức sau mỗi lần AI bóc tách thành công.
- **Giao diện Cao cấp**: Hỗ trợ chế độ Sáng/Tối (Light/Dark Mode) mượt mà.

### 2. Dành cho Quản trị viên (Admin)
- **Admin Dashboard**: Nơi quản lý toàn bộ danh sách người dùng trên hệ thống, thống kê số lượng giao dịch của từng người.
- **"God Mode" (Chế độ xem hộ)**: Admin có thể "xem hộ" (View) tài khoản của một người dùng bất kỳ, soi được cả biểu đồ chi tiêu và lịch sử chat của họ (Chat được vô hiệu hóa để đảm bảo Admin không nhắn nhầm).
- **Xóa Dữ liệu An toàn (Cascade Delete)**: Quản trị viên có thể xóa người dùng vi phạm. Hệ thống tự động xóa dọn sạch sẽ toàn bộ Giao dịch và Tin nhắn của người dùng đó khỏi hệ thống để giải phóng bộ nhớ.

### 3. Kiến trúc Chịu tải Cao (Enterprise Architecture)
- **Asynchronous Job Queue (Hàng đợi Bất đồng bộ)**: Trái tim của Backend. Khi 1000 người dùng gửi ảnh cùng lúc, Backend sẽ tiếp nhận toàn bộ (0.01 giây/request) và đưa vào Hàng đợi (Queue), không làm treo giao diện người dùng. Frontend sẽ sử dụng kỹ thuật **Polling** để kiểm tra tiến độ.
- **Global Rate Limiter**: Hệ thống tự động điều tiết lưu lượng bằng Token Bucket (1 request mỗi 4 giây) để đảm bảo không bao giờ vi phạm giới hạn Quota của Google AI (15 request/phút), triệt tiêu hoàn toàn lỗi 429 Quota Exceeded.
- **Backend API Key Rotation**: Hỗ trợ cấu hình nhiều API Key. Backend sẽ tự động luân chuyển (rotate) giữa các chìa khóa nếu xảy ra rủi ro lỗi từ Google, giúp hệ thống không bao giờ bị "đứng hình" vì hết Quota.

---

## 🚀 Hướng dẫn Triển khai và Trải nghiệm (How to Run)

### Cách 1: Trải nghiệm Demo Trực tuyến (Nếu có)
Nếu dự án đang được host trên Render, bạn có thể truy cập thẳng vào URL được cung cấp (VD: `https://finance-agent-*.onrender.com`).
1. Tạo một tài khoản mới và đăng nhập.
2. Tại màn hình Chat, nhập `Hôm nay ăn phở 50k` hoặc Tải lên một bức ảnh chuyển khoản.
3. Xem AI phân tích và quay lại tab "Tổng quan" để xem biểu đồ.

### Cách 2: Triển khai trên máy tính cá nhân (Local)
1. **Clone mã nguồn:**
   ```bash
   git clone https://github.com/NamHaiIT2HUST/Finance-Agent.git
   cd Finance-Agent
   ```
2. **Cấu hình môi trường (.env):**
   Tạo file `.env` ở thư mục gốc và cung cấp Google Gemini API Key. Bạn có thể sử dụng nhiều key để hệ thống tự xoay vòng chống nghẽn:
   ```env
   GEMINI_API_KEY=key_1,key_2,key_3
   ```
3. **Cài đặt thư viện và Khởi chạy:**
   Yêu cầu: Cài đặt sẵn ngôn ngữ [Go (Golang)](https://go.dev/dl/).
   ```bash
   go mod tidy
   go run cmd/agent/main.go
   ```
4. **Truy cập ứng dụng:**
   Mở trình duyệt và truy cập `http://localhost:8080`.
   - Để trải nghiệm tài khoản Admin: Cần chỉnh sửa thủ công trường `role` trong CSDL SQLite (`finance.db`) của tài khoản từ `user` thành `admin`.

---
*Kiến trúc hệ thống được xây dựng hoàn toàn hướng đến sự "tối giản cho người dùng" nhưng cực kỳ "phức tạp và mạnh mẽ ở Backend" - Tự hào thiết kế để chạy mượt mà ngay cả khi có hàng ngàn truy cập đồng thời!*
