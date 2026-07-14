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
	"strings"
	"time"

	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/db"
	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/tools"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

var jwtKey = []byte("super_secret_finance_key_2026")

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// CheckAuth Middleware
func getAuthUserID(r *http.Request) (int, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, fmt.Errorf("missing token")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}
	return claims.UserID, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Lưu ý: Không tìm thấy file .env")
	}

	// Initialize SQLite Database
	db.InitDB("./finance.db")

	ctx := context.Background()
	aiClient, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatalf("Lỗi khởi tạo Gemini: %v", err)
	}
	defer aiClient.Close()

	// API Đăng ký
	http.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&creds)

		if creds.Username == "" || creds.Password == "" {
			http.Error(w, `{"success": false, "error": "Thiếu username/password"}`, http.StatusBadRequest)
			return
		}

		err := db.CreateUser(creds.Username, creds.Password)
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
		if err != nil || !db.CheckPassword(user.Password, creds.Password) {
			http.Error(w, `{"success": false, "error": "Sai thông tin đăng nhập"}`, http.StatusUnauthorized)
			return
		}

		// Tạo JWT Token
		expirationTime := time.Now().Add(24 * time.Hour)
		claims := &Claims{
			UserID:   user.ID,
			Username: user.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString(jwtKey)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   tokenString,
			"user":    map[string]interface{}{"id": user.ID, "username": user.Username},
		})
	})

	// Lấy danh sách chi tiêu (của User đang đăng nhập)
	http.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		userID, err := getAuthUserID(r)
		if err != nil {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		expenses, err := db.GetExpensesByUser(userID)
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
		userID, errAuth := getAuthUserID(r)
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
			mimeType := "image/jpeg"
			if strings.HasSuffix(strings.ToLower(header.Filename), ".png") {
				mimeType = "image/png"
			} else if strings.HasSuffix(strings.ToLower(header.Filename), ".webp") {
				mimeType = "image/webp"
			}
			promptParts = append(promptParts, genai.ImageData(mimeType, imgBytes))

			if userText == "" {
				userText = "Hãy trích xuất thông tin chi tiêu từ hóa đơn/ảnh chụp màn hình này."
			}
		} else if userText == "" {
			http.Error(w, `{"error": "Vui lòng nhập văn bản hoặc chọn ảnh"}`, http.StatusBadRequest)
			return
		}

		promptParts = append(promptParts, genai.Text(userText))

		model := aiClient.GenerativeModel("gemini-3.5-flash")
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{
				genai.Text(`Bạn là một Agent quản lý tài chính cá nhân.
Người dùng sẽ nói về các khoản thu nhập hoặc chi tiêu. Nếu người dùng gửi hóa đơn siêu thị dài, hãy bóc tách TỪNG MÓN HÀNG thành các khoản chiêng biệt.
Nhiệm vụ: Phân tích và CHỈ trả về một MẢNG (ARRAY) JSON, mỗi phần tử có các key: 
- "date" (YYYY-MM-DD)
- "type" ("Thu" hoặc "Chi")
- "amount" (số nguyên dương)
- "category" (nhóm chi tiêu/thu nhập)
- "description".
Ví dụ: [{"date": "2023-10-25", "type": "Chi", "amount": 50000, "category": "Ăn uống", "description": "Phở"}, {"date": "2023-10-25", "type": "Chi", "amount": 10000, "category": "Ăn uống", "description": "Trà đá"}]
TUYỆT ĐỐI trả về mảng JSON hợp lệ. Không giải thích gì thêm.`),
			},
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

		resp, errGen := model.GenerateContent(ctx, promptParts...)
		if errGen != nil {
			errMsg := "❌ Lỗi AI: " + errGen.Error()
			if strings.Contains(errGen.Error(), "429") || strings.Contains(errGen.Error(), "Quota") {
				errMsg = "Hệ thống AI đang quá tải, vui lòng thử lại sau."
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "reply": errMsg})
			return
		}

		var replyText string
		for _, part := range resp.Candidates[0].Content.Parts {
			replyText = fmt.Sprintf("%v", part)
		}

		// Dọn dẹp sơ nếu AI vẫn cố chấp trả về markdown
		replyText = strings.TrimSpace(replyText)
		replyText = strings.TrimPrefix(replyText, "```json\n")
		replyText = strings.TrimPrefix(replyText, "```json")
		replyText = strings.TrimSuffix(replyText, "\n```")
		replyText = strings.TrimSuffix(replyText, "```")

		// Dùng regex để xóa triệt để dấu phẩy thừa trước ngoặc đóng (trailing commas)
		re := regexp.MustCompile(`,\s*([\]}])`)
		replyText = re.ReplaceAllString(replyText, "$1")

		exps, errParse := tools.ParseExpensesJSON(replyText)
		if errParse != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "reply": "Lỗi đọc JSON: " + errParse.Error()})
			return
		}

		replyStr := "✅ Đã ghi vào sổ:\n"
		totalAdded := 0
		var finalExps []db.Expense
		for _, exp := range exps {
			dbExp := db.Expense{
				UserID:      userID,
				Date:        exp.Date,
				Type:        exp.Type,
				Amount:      exp.Amount,
				Category:    exp.Category,
				Description: exp.Description,
			}
			errSheet := db.AddExpense(userID, &dbExp)
			if errSheet != nil {
				continue
			}
			finalExps = append(finalExps, dbExp)
			replyStr += fmt.Sprintf("- [%s] %s - %d đ (%s)\n", exp.Type, exp.Description, exp.Amount, exp.Category)
			totalAdded += exp.Amount
		}
		replyStr += fmt.Sprintf("\n💰 Tổng cộng: %d đ", totalAdded)

		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "reply": replyStr, "expenses": finalExps})
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
