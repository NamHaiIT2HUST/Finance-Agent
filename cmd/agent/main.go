package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/tools"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// Check Web Password
func checkAuth(r *http.Request) bool {
	expectedPwd := os.Getenv("WEB_PASSWORD")
	if expectedPwd == "" {
		return true // No password set
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	return token == expectedPwd
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Lưu ý: Không tìm thấy file .env (Bỏ qua vì đang chạy trên Cloud)")
	}

	ctx := context.Background()
	aiClient, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatalf("Lỗi khởi tạo Gemini: %v", err)
	}
	defer aiClient.Close()

	// API để xác thực
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		if !checkAuth(r) {
			http.Error(w, `{"success": false, "error": "Sai mật khẩu!"}`, http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	// API lấy danh sách chi tiêu
	http.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		
		if !checkAuth(r) {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		userSpreadsheet := os.Getenv("SPREADSHEET_ID")
		expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
		if err != nil {
			http.Error(w, `{"error": "Không thể tải dữ liệu"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(expenses)
	})

	// API Chat & Xử lý hình ảnh
	http.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		
		if !checkAuth(r) {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		err := r.ParseMultipartForm(10 << 20) // 10MB max
		if err != nil {
			http.Error(w, `{"error": "Lỗi parse form"}`, http.StatusBadRequest)
			return
		}

		userText := r.FormValue("text")
		
		var promptParts []genai.Part

		// Xử lý file ảnh nếu có
		file, header, errFile := r.FormFile("image")
		if errFile == nil {
			defer file.Close()
			imgBytes, _ := io.ReadAll(file)
			
			// Detect MIME type
			mimeType := "jpeg"
			if strings.HasSuffix(strings.ToLower(header.Filename), ".png") {
				mimeType = "png"
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

		// Khởi tạo Model
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
TUYỆT ĐỐI trả về mảng JSON hợp lệ, KHÔNG có dấu phẩy thừa (trailing comma) ở phần tử cuối cùng.
Không giải thích gì thêm.`),
			},
		}
		model.ResponseMIMEType = "application/json"

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

		replyText = strings.TrimPrefix(replyText, "```json\n")
		replyText = strings.TrimSuffix(replyText, "\n```")
		replyText = strings.TrimSuffix(replyText, "```")
		replyText = strings.ReplaceAll(replyText, ",]", "]")
		replyText = strings.ReplaceAll(replyText, ", }", "}")
		replyText = strings.ReplaceAll(replyText, ",}", "}")
		replyText = strings.ReplaceAll(replyText, " \n", "")

		exps, errParse := tools.ParseExpensesJSON(replyText)
		if errParse != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "reply": "Lỗi đọc JSON: " + errParse.Error()})
			return
		}

		userSpreadsheet := os.Getenv("SPREADSHEET_ID")
		errSheet := tools.AppendExpensesToSheet(userSpreadsheet, exps)
		
		if errSheet != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "reply": "Lỗi Database: " + errSheet.Error()})
			return
		}

		replyStr := "✅ Đã ghi vào sổ:\n"
		totalAdded := 0
		for _, exp := range exps {
			replyStr += fmt.Sprintf("- [%s] %s - %d VND (%s)\n", exp.Type, exp.Description, exp.Amount, exp.Category)
			totalAdded += exp.Amount
		}
		replyStr += fmt.Sprintf("\n💰 Tổng cộng: %d VND", totalAdded)

		// Cảnh báo ngân sách
		budgetStr := os.Getenv("MONTHLY_BUDGET")
		if budgetStr != "" {
			if budget, errB := strconv.Atoi(budgetStr); errB == nil && budget > 0 {
				allExps, errF := tools.FetchExpensesFromSheet(userSpreadsheet)
				if errF == nil {
					currentMonth := time.Now().Format("2006-01")
					totalMonthExpense := 0
					
					for _, e := range allExps {
						if (e.Type == "Chi" || e.Type == "chi" || e.Type == "") && strings.HasPrefix(e.Date, currentMonth) {
							totalMonthExpense += e.Amount
						}
					}
					
					percent := float64(totalMonthExpense) / float64(budget) * 100
					if percent >= 100 {
						replyStr += fmt.Sprintf("\n\n🚨 BÁO ĐỘNG ĐỎ: Đã tiêu %d VND, vượt quá 100%% ngân sách tháng (%d VND)!", totalMonthExpense, budget)
					} else if percent >= 90 {
						replyStr += fmt.Sprintf("\n\n⚠️ CẢNH BÁO: Đã tiêu %d VND (%.1f%% ngân sách).", totalMonthExpense, percent)
					}
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "reply": replyStr, "expenses": exps})
	})

	// Phục vụ giao diện Web tĩnh
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
