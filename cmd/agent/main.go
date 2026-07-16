package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/db"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

var jwtKey = []byte("super_secret_finance_key_2026")

// Cấu hình xoay vòng (Rotate) API Keys để chống Quá Tải
var apiKeys []string
var currentKeyIndex int
var keyMutex sync.Mutex

// Cơ chế Rate Limiter toàn cục (Đảm bảo không vượt quá 15 request/phút của bản Free)
var aiRateMutex sync.Mutex
var lastAIRequest time.Time

func waitGlobalRateLimit() {
	aiRateMutex.Lock()
	defer aiRateMutex.Unlock()
	
	elapsed := time.Since(lastAIRequest)
	if elapsed < 4*time.Second {
		time.Sleep(4*time.Second - elapsed)
	}
	lastAIRequest = time.Now()
}

func getNextAPIKey() string {
	keyMutex.Lock()
	defer keyMutex.Unlock()
	if len(apiKeys) == 0 {
		return os.Getenv("GEMINI_API_KEY")
	}
	key := apiKeys[currentKeyIndex]
	currentKeyIndex = (currentKeyIndex + 1) % len(apiKeys)
	return key
}

// Khai báo cấu trúc Job cho Hàng đợi Bất đồng bộ
type ChatJob struct {
	UserID      int
	UserText    string
	PromptParts []genai.Part
	AIMessageID int // ID của tin nhắn chờ (pending) trong Database
}

var jobQueue = make(chan ChatJob, 1000)

func startAIWorker() {
	for job := range jobQueue {
		processChatJob(job)
	}
}

func processChatJob(job ChatJob) {
	ctx := context.Background()

	currentDate := time.Now().Format("2006-01-02")
	promptText := fmt.Sprintf(`Hôm nay là ngày %s. Bạn là một Agent quản lý tài chính cá nhân tinh tế và thông minh.
Người dùng sẽ gửi tin nhắn hoặc ảnh hóa đơn, bill chuyển khoản. Nếu không rõ ngày, MẶC ĐỊNH dùng ngày hôm nay (%s).
PHÂN LOẠI DANH MỤC THÔNG MINH theo danh sách BẮT BUỘC sau:
- CHI: "Ăn uống", "Di chuyển", "Hóa đơn & Tiện ích" (điện, nước, internet...), "Mua sắm", "Giải trí", "Sức khỏe", "Chuyển tiền".
- THU: "Lương", "Thưởng", "Kinh doanh", "Được tặng", "Thu nhập khác".
*Chú ý đặc biệt: Tiền điện, tiền nước phải thuộc "Hóa đơn & Tiện ích", tuyệt đối không phân vào "Ăn uống". Nếu là mua vé xem phim thì là "Giải trí".
Nhiệm vụ: Phân tích và CHỈ trả về một MẢNG (ARRAY) JSON có các key: 
- "date" (YYYY-MM-DD)
- "type" ("Thu" hoặc "Chi")
- "amount" (số nguyên dương)
- "category" (1 trong các danh mục trên)
- "description" (Tóm tắt ngắn gọn, dễ hiểu).
Ví dụ: [{"date": "%s", "type": "Chi", "amount": 10000, "category": "Hóa đơn & Tiện ích", "description": "Đóng tiền nước"}]
TUYỆT ĐỐI trả về mảng JSON hợp lệ. Không giải thích gì thêm.`, currentDate, currentDate, currentDate)

	var resp *genai.GenerateContentResponse
	var errGen error

	maxRetries := len(apiKeys)
	if maxRetries < 1 {
		maxRetries = 1
	}

	for i := 0; i < maxRetries; i++ {
		apiKey := getNextAPIKey()
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			continue
		}
		
		model := client.GenerativeModel("gemini-3.5-flash")
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(promptText)},
		}
		model.ResponseMIMEType = "application/json"
		model.ResponseSchema = &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"date":        {Type: genai.TypeString},
					"type":        {Type: genai.TypeString},
					"amount":      {Type: genai.TypeInteger},
					"category":    {Type: genai.TypeString},
					"description": {Type: genai.TypeString},
				},
				Required: []string{"date", "type", "amount", "category", "description"},
			},
		}

		waitGlobalRateLimit() // Điều tiết 15 RPM ở đây!
		resp, errGen = model.GenerateContent(ctx, job.PromptParts...)
		client.Close()

		if errGen == nil {
			break
		}
		if !strings.Contains(errGen.Error(), "429") && !strings.Contains(errGen.Error(), "Quota") {
			break
		}
	}

	if errGen != nil {
		errMsg := "❌ Lỗi AI: " + errGen.Error()
		// Nếu hết quota thật sự từ Google, ta sẽ ngủ 20s và Đẩy lại vào Hàng đợi để nó thử lại cho đến khi thành công!
		if strings.Contains(errGen.Error(), "429") || strings.Contains(errGen.Error(), "Quota") {
			log.Println("Hệ thống Google AI đang tạm đầy, hệ thống tự động Sleep 20s và Retry lại...")
			time.Sleep(20 * time.Second)
			jobQueue <- job // Push back to queue!
			return
		}
		db.UpdateMessage(job.AIMessageID, errMsg, "error")
		return
	}

	var replyText string
	for _, part := range resp.Candidates[0].Content.Parts {
		replyText = fmt.Sprintf("%v", part)
	}

	replyText = strings.TrimSpace(replyText)
	replyText = strings.TrimPrefix(replyText, "```json\n")
	replyText = strings.TrimPrefix(replyText, "```json")
	replyText = strings.TrimSuffix(replyText, "\n```")
	replyText = strings.TrimSuffix(replyText, "```")

	re := regexp.MustCompile(`,\s*([\]}])`)
	replyText = re.ReplaceAllString(replyText, "$1")

	var expenses []db.Expense
	errJSON := json.Unmarshal([]byte(replyText), &expenses)
	if errJSON != nil {
		db.UpdateMessage(job.AIMessageID, "❌ Lỗi đọc dữ liệu AI: "+errJSON.Error()+"\nRaw: "+replyText, "error")
		return
	}

	totalAmount := 0
	var replyLines []string
	for _, exp := range expenses {
		if exp.Amount <= 0 || exp.Date == "" || exp.Type == "" {
			continue
		}
		errSave := db.AddExpense(job.UserID, &exp)
		if errSave != nil {
			log.Printf("Lỗi lưu DB: %v", errSave)
			continue
		}
		totalAmount += exp.Amount
		emoji := "🔴"
		if exp.Type == "Thu" {
			emoji = "🟢"
		}
		replyLines = append(replyLines, fmt.Sprintf("- %s [%s] %s - %d đ (%s)", emoji, exp.Type, exp.Description, exp.Amount, exp.Category))
	}

	if len(replyLines) == 0 {
		db.UpdateMessage(job.AIMessageID, "❌ Không tìm thấy khoản thu/chi nào hợp lệ.", "error")
		return
	}

	replyStr := "✅ Đã ghi vào sổ:\n" + strings.Join(replyLines, "\n") + fmt.Sprintf("\n🔒 Tổng cộng: %d đ", totalAmount)
	db.UpdateMessage(job.AIMessageID, replyStr, "completed")
}

type Claims struct {
	UserID   int    `json:"user_id"`
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// CheckAuth Middleware
func getAuthUser(r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing token")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Lưu ý: Không tìm thấy file .env")
	}

	// Initialize Database
	db.InitDB("./finance.db")
	db.FailPendingMessages() // Dọn dẹp các Job bị kẹt do server restart

	// Parse API Keys từ biến môi trường (Hỗ trợ cấu hình nhiều key để xoay vòng)
	keyStr := os.Getenv("GEMINI_API_KEY")
	if keyStr != "" {
		keys := strings.Split(keyStr, ",")
		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k != "" {
				apiKeys = append(apiKeys, k)
			}
		}
	}
	if len(apiKeys) == 0 {
		log.Println("Lưu ý: Chưa cấu hình GEMINI_API_KEY hợp lệ")
	}

	// Khởi chạy Hàng đợi Xử lý AI Bất đồng bộ
	go startAIWorker()

	// API Đăng ký
	http.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var creds struct {
			FullName string `json:"full_name"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&creds)

		if creds.FullName == "" || creds.Username == "" || creds.Password == "" {
			http.Error(w, `{"success": false, "error": "Thiếu thông tin"}`, http.StatusBadRequest)
			return
		}

		err := db.CreateUser(creds.FullName, creds.Username, creds.Password)
		if err != nil {
			http.Error(w, `{"success": false, "error": "Tên đăng nhập đã tồn tại"}`, http.StatusConflict)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	// API Đăng nhập
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&creds)

		user, err := db.GetUserByUsername(creds.Username)
		if err != nil || !db.CheckPassword(user, creds.Password) {
			http.Error(w, `{"success": false, "error": "Sai thông tin đăng nhập"}`, http.StatusUnauthorized)
			return
		}

		// Tạo JWT Token
		expirationTime := time.Now().Add(24 * time.Hour)
		claims := &Claims{
			UserID:   user.ID,
			FullName: user.FullName,
			Username: user.Username,
			Role:     user.Role,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString(jwtKey)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   tokenString,
			"user":    map[string]interface{}{"id": user.ID, "full_name": user.FullName, "username": user.Username, "role": user.Role},
		})
	})

	// Lấy danh sách chi tiêu (của User đang đăng nhập)
	http.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user, err := getAuthUser(r)
		if err != nil {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		expenses, err := db.GetExpensesByUser(user.UserID)
		if err != nil {
			http.Error(w, `{"error": "Không thể tải dữ liệu"}`, http.StatusInternalServerError)
			return
		}
		// Xử lý list rỗng để React/JS khỏi dính null
		if expenses == nil {
			expenses = []db.Expense{}
		}
		json.NewEncoder(w).Encode(expenses)
	})

	// API Chat & AI
	http.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user, errAuth := getAuthUser(r)
		if errAuth != nil {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, `{"error": "Lỗi parse form"}`, http.StatusBadRequest)
			return
		}

		userText := r.FormValue("text")
		var promptParts []genai.Part

		file, header, errFile := r.FormFile("image")
		if errFile == nil {
			defer file.Close()
			imgBytes, _ := io.ReadAll(file)
			mimeType := "jpeg"
			if strings.HasSuffix(strings.ToLower(header.Filename), ".png") {
				mimeType = "png"
			} else if strings.HasSuffix(strings.ToLower(header.Filename), ".webp") {
				mimeType = "webp"
			} else if strings.HasSuffix(strings.ToLower(header.Filename), ".heic") {
				mimeType = "heic"
			}
			promptParts = append(promptParts, genai.ImageData(mimeType, imgBytes))

			if userText == "" {
				userText = "Hãy trích xuất thông tin chi tiêu từ hóa đơn/ảnh chụp màn hình này."
			}
		} else if userText == "" {
			http.Error(w, `{"error": "Vui lòng nhập văn bản hoặc chọn ảnh"}`, http.StatusBadRequest)
			return
		}

		// Lưu tin nhắn của User vào DB
		savedUserText := userText
		if errFile == nil {
			if r.FormValue("text") == "" {
				savedUserText = "📷 Đã gửi một ảnh"
			} else {
				savedUserText = "📷 Đã gửi một ảnh: " + r.FormValue("text")
			}
		}
		db.SaveMessage(user.UserID, true, savedUserText, "completed")

		promptParts = append(promptParts, genai.Text(userText))

		// Lưu một tin nhắn AI giả vào DB trạng thái pending
		pendingMsg, _ := db.SaveMessage(user.UserID, false, "⏳ AI đang phân tích (Bạn đang ở trong hàng đợi)...", "pending")

		// Đẩy vào hàng đợi để worker xử lý (non-blocking)
		jobQueue <- ChatJob{
			UserID:      user.UserID,
			UserText:    userText,
			PromptParts: promptParts,
			AIMessageID: pendingMsg.ID,
		}

		// Trả về ngay cho Frontend
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message_id": pendingMsg.ID,
		})
	})

	// API Polling trạng thái tin nhắn
	http.HandleFunc("/api/chat/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user, errAuth := getAuthUser(r)
		if errAuth != nil {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		msgIDStr := r.URL.Query().Get("id")
		msgID, err := strconv.Atoi(msgIDStr)
		if err != nil {
			http.Error(w, `{"error": "Invalid ID"}`, http.StatusBadRequest)
			return
		}

		msg, err := db.GetMessageByID(msgID)
		if err != nil {
			http.Error(w, `{"error": "Not found"}`, http.StatusNotFound)
			return
		}

		if msg.UserID != user.UserID && user.Role != "admin" {
			http.Error(w, `{"error": "Forbidden"}`, http.StatusForbidden)
			return
		}

		json.NewEncoder(w).Encode(msg)
	})

	// History API
	http.HandleFunc("/api/chat/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user, errAuth := getAuthUser(r)
		if errAuth != nil {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		targetUserID := user.UserID
		if user.Role == "admin" {
			uidStr := r.URL.Query().Get("user_id")
			if uidStr != "" {
				uid, err := strconv.Atoi(uidStr)
				if err == nil {
					targetUserID = uid
				}
			}
		}

		messages, err := db.GetMessagesByUser(targetUserID)
		if err != nil {
			http.Error(w, `{"error": "Lỗi lấy lịch sử"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(messages)
	})

	// Admin API
	http.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user, err := getAuthUser(r)
		if err != nil || user.Role != "admin" {
			http.Error(w, `{"error": "Forbidden"}`, http.StatusForbidden)
			return
		}

		if r.Method == "GET" {
			// Nếu có tham số user_id thì trả về expenses của user đó (tính năng Admin soi tài khoản)
			userIDStr := r.URL.Query().Get("user_id")
			if userIDStr != "" {
				targetUserID, err := strconv.Atoi(userIDStr)
				if err != nil {
					http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
					return
				}
				
				// Lấy expenses của user mục tiêu
				exps, err := db.GetExpensesByUser(targetUserID)
				if err != nil {
					http.Error(w, `{"error": "Không thể lấy dữ liệu chi tiêu"}`, http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(exps)
				return
			}

			// Không có user_id thì trả về danh sách tất cả user
			users, err := db.GetAllUsers()
			if err != nil {
				http.Error(w, `{"error": "Lỗi lấy dữ liệu"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(users)
		} else if r.Method == "DELETE" {
			var payload struct {
				ID int `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.ID == user.UserID {
				http.Error(w, `{"error": "Không thể tự xóa chính mình"}`, http.StatusBadRequest)
				return
			}
			err := db.DeleteUser(payload.ID)
			if err != nil {
				http.Error(w, `{"error": "Lỗi xóa user"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
		}
	})

	http.Handle("/", http.FileServer(http.Dir("./web")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🌐 Standalone Web App đang chạy ở cổng %s", port)
	if errHttp := http.ListenAndServe(":"+port, nil); errHttp != nil {
		log.Fatalf("Lỗi khởi động HTTP Server: %v", errHttp)
	}
}
