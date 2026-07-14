package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/tools"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"google.golang.org/api/option"
)

type WebAppUser struct {
	ID int64 `json:"id"`
}

func validateTelegramWebAppData(initData, token string) (map[string]string, error) {
	q, err := url.ParseQuery(initData)
	if err != nil {
		return nil, err
	}

	hash := q.Get("hash")
	if hash == "" {
		return nil, fmt.Errorf("missing hash")
	}
	q.Del("hash")

	var keys []string
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataCheckArr []string
	for _, k := range keys {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("%s=%s", k, q.Get(k)))
	}
	dataCheckString := strings.Join(dataCheckArr, "\n")

	secretKeyMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyMac.Write([]byte(token))
	secretKey := secretKeyMac.Sum(nil)

	hashMac := hmac.New(sha256.New, secretKey)
	hashMac.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(hashMac.Sum(nil))

	if expectedHash != hash {
		return nil, fmt.Errorf("invalid hash")
	}

	res := make(map[string]string)
	for k, v := range q {
		res[k] = v[0]
	}
	return res, nil
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
Không giải thích gì thêm.`),
		},
	}
	model.ResponseMIMEType = "application/json"

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Lỗi khởi tạo Telegram Bot: %v", err)
	}
	bot.Debug = false
	log.Printf("🤖 Đã đăng nhập thành công vào bot: %s", bot.Self.UserName)

	loc, errLoc := time.LoadLocation("Asia/Ho_Chi_Minh")
	if errLoc != nil {
		log.Printf("Lỗi load timezone: %v", errLoc)
	}
	// Khởi tạo Cron với Timezone Việt Nam
	c := cron.New(cron.WithLocation(loc))

	// Đặt lịch vào 22:00 (10 giờ tối) mỗi ngày
	_, errCron := c.AddFunc("0 22 * * *", func() {
		// Lấy danh sách người dùng được cấp quyền
		authUsers := os.Getenv("AUTHORIZED_USERS")
		var chatIDs []int64
		if authUsers != "" {
			for _, idStr := range strings.Split(authUsers, ",") {
				idStr = strings.TrimSpace(idStr)
				if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
					chatIDs = append(chatIDs, id)
				}
			}
		} else {
			if chatIDStr := os.Getenv("CHAT_ID"); chatIDStr != "" {
				if id, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
					chatIDs = append(chatIDs, id)
				}
			}
		}

		today := time.Now().In(loc)
		tomorrow := today.AddDate(0, 0, 1)
		isLastDayOfMonth := today.Month() != tomorrow.Month()
		isSunday := today.Weekday() == time.Sunday

		for _, chatID := range chatIDs {
			chatIDStr := strconv.FormatInt(chatID, 10)
			userSpreadsheet := os.Getenv("SPREADSHEET_ID_" + chatIDStr)
			if userSpreadsheet == "" {
				userSpreadsheet = os.Getenv("SPREADSHEET_ID")
			}

			expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
			if err != nil {
				continue
			}

			totalIncome := 0
			totalExpense := 0
			var reportText string

			if isLastDayOfMonth {
				// Báo cáo cuối tháng
				currentMonth := today.Format("2006-01")
				for _, e := range expenses {
					if strings.HasPrefix(e.Date, currentMonth) {
						if e.Type == "Thu" || e.Type == "thu" {
							totalIncome += e.Amount
						} else {
							totalExpense += e.Amount
						}
					}
				}
				netBalance := totalIncome - totalExpense

				// Gọi AI viết báo cáo
				prompt := fmt.Sprintf("Tôi vừa kết thúc tháng %s. Tổng thu: %d, Tổng chi: %d, Số dư: %d. Hãy viết báo cáo tài chính cuối tháng súc tích, chuyên nghiệp bằng tiếng Việt.", today.Format("01/2006"), totalIncome, totalExpense, netBalance)
				
				origInst := model.SystemInstruction
				model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text("Bạn là chuyên gia tài chính. Trả lời ngắn gọn.")}}
				model.ResponseMIMEType = "text/plain"
				resp, errAI := model.GenerateContent(ctx, genai.Text(prompt))
				model.SystemInstruction = origInst
				model.ResponseMIMEType = "application/json"

				aiText := ""
				if errAI == nil && resp != nil {
					for _, part := range resp.Candidates[0].Content.Parts {
						aiText += fmt.Sprintf("%v", part)
					}
				}
				
				reportText = fmt.Sprintf("🗓 **BÁO CÁO TÀI CHÍNH CUỐI THÁNG**\n\n🟢 Tổng Thu: %d đ\n🔴 Tổng Chi: %d đ\n💰 Số Dư: %d đ\n\n💡 Nhận xét từ AI:\n%s", totalIncome, totalExpense, netBalance, aiText)

			} else if isSunday {
				// Báo cáo cuối tuần
				weekAgo := today.AddDate(0, 0, -6).Format("2006-01-02")
				for _, e := range expenses {
					if e.Date >= weekAgo && e.Date <= today.Format("2006-01-02") {
						if e.Type == "Thu" || e.Type == "thu" {
							totalIncome += e.Amount
						} else {
							totalExpense += e.Amount
						}
					}
				}
				reportText = fmt.Sprintf("📅 **BÁO CÁO CUỐI TUẦN**\n\nTuần qua bạn đã thu vào %d đ và chi tiêu %d đ.\nNghỉ ngơi thật tốt để chuẩn bị cho tuần mới nhé!", totalIncome, totalExpense)
			} else {
				// Báo cáo cuối ngày
				todayStr := today.Format("2006-01-02")
				for _, e := range expenses {
					if e.Date == todayStr {
						if e.Type == "Thu" || e.Type == "thu" {
							totalIncome += e.Amount
						} else {
							totalExpense += e.Amount
						}
					}
				}
				if totalIncome == 0 && totalExpense == 0 {
					reportText = "🔔 Ting ting! Hôm nay có vẻ bạn không phát sinh giao dịch nào. Nếu có quên thì nhập sổ đi nhé!"
				} else {
					reportText = fmt.Sprintf("🌙 **TỔNG KẾT HÔM NAY**\n\n🟢 Đã thu: %d đ\n🔴 Đã chi: %d đ\nBạn còn sót khoản nào chưa nhập không?", totalIncome, totalExpense)
				}
			}

			msg := tgbotapi.NewMessage(chatID, reportText)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}
		log.Println("⏰ Đã chạy lịch gửi báo cáo tự động!")
	})

	if errCron != nil {
		log.Printf("Lỗi cài đặt lịch: %v", errCron)
	}
	c.Start()

	// Cấu hình HTTP Server cho API và Web Dashboard (Phục vụ cả Local và Webhook)
	http.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		
		initData := r.Header.Get("X-Telegram-Init-Data")
		if initData == "" {
			http.Error(w, `{"error": "Unauthorized: Missing initData"}`, http.StatusUnauthorized)
			return
		}

		botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
		parsedData, errAuth := validateTelegramWebAppData(initData, botToken)
		if errAuth != nil {
			http.Error(w, `{"error": "Unauthorized: Invalid hash"}`, http.StatusUnauthorized)
			return
		}

		userJSON := parsedData["user"]
		var webUser WebAppUser
		if err := json.Unmarshal([]byte(userJSON), &webUser); err != nil {
			http.Error(w, `{"error": "Unauthorized: Invalid user data"}`, http.StatusUnauthorized)
			return
		}

		chatIDStr := strconv.FormatInt(webUser.ID, 10)
		
		// Phân quyền
		authUsers := os.Getenv("AUTHORIZED_USERS")
		if authUsers != "" && !strings.Contains(authUsers, chatIDStr) {
			http.Error(w, `{"error": "Forbidden: User not allowed"}`, http.StatusForbidden)
			return
		}

		userSpreadsheet := os.Getenv("SPREADSHEET_ID_" + chatIDStr)
		if userSpreadsheet == "" {
			userSpreadsheet = os.Getenv("SPREADSHEET_ID")
		}

		expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
		if err != nil {
			http.Error(w, `{"error": "không thể tải dữ liệu"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(expenses)
	})

	http.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.Dir("./web"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		log.Printf("🌐 Đang chạy HTTP Server ở cổng %s", port)
		if errHttp := http.ListenAndServe(":"+port, nil); errHttp != nil {
			log.Printf("Lỗi khởi động HTTP Server (hoặc server đã chạy): %v", errHttp)
		}
	}()

	var updates tgbotapi.UpdatesChannel
	webhookURL := os.Getenv("WEBHOOK_URL")

	if webhookURL != "" {
		// Đăng ký Webhook với Telegram (nên dùng một path khó đoán như /<token> hoặc /webhook)
		// Ở đây vẫn để / để tương thích cũ
		wh, _ := tgbotapi.NewWebhook(webhookURL + "/")
		_, errWh := bot.Request(wh)
		if errWh != nil {
			log.Fatalf("Lỗi set webhook: %v", errWh)
		}

		// Lắng nghe qua channel Webhook
		updates = bot.ListenForWebhook("/")
		fmt.Printf("🚀 Agent đang chờ tin nhắn (Chế độ Webhook tại %s)...\n", webhookURL)
	} else {
		// Fallback về Long Polling nếu chạy ở local
		_, _ = bot.Request(tgbotapi.DeleteWebhookConfig{})
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates = bot.GetUpdatesChan(u)
		fmt.Println("🚀 Agent đang chờ tin nhắn (Chế độ Long Polling)...")
	}

	editState := make(map[int64]int)

	for update := range updates {
		if update.CallbackQuery != nil {
			chatID := update.CallbackQuery.Message.Chat.ID
			data := update.CallbackQuery.Data

			if strings.HasPrefix(data, "edit_") {
				rowStr := strings.TrimPrefix(data, "edit_")
				rowIndex, _ := strconv.Atoi(rowStr)
				editState[chatID] = rowIndex

				bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✏️ Bạn đang sửa giao dịch ở dòng %d. Vui lòng nhắn nội dung hoặc gửi hóa đơn mới để tôi cập nhật lại nhé!", rowIndex)))
				
				// Trả lời Callback để tắt icon loading trên nút
				bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Đã chọn giao dịch"))
			}
			continue
		}

		if update.Message == nil {
			continue
		}

		chatIDStr := strconv.FormatInt(update.Message.Chat.ID, 10)

		// Kiểm tra Bảo mật Đa người dùng
		authUsers := os.Getenv("AUTHORIZED_USERS")
		if authUsers != "" && !strings.Contains(authUsers, chatIDStr) {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ CẢNH BÁO: Bạn không có quyền truy cập vào Bot này!"))
			continue
		}

		// Xác định SPREADSHEET_ID cho người dùng hiện tại
		userSpreadsheet := os.Getenv("SPREADSHEET_ID_" + chatIDStr)
		if userSpreadsheet == "" {
			userSpreadsheet = os.Getenv("SPREADSHEET_ID")
		}

		if update.Message.Text == "/report" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Đang tổng hợp dữ liệu..."))

			expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi đọc dữ liệu: "+err.Error()))
				continue
			}

			totalExpense := 0
			totalIncome := 0
			for _, exp := range expenses {
				if exp.Type == "Thu" || exp.Type == "thu" {
					totalIncome += exp.Amount
				} else {
					totalExpense += exp.Amount
				}
			}
			netBalance := totalIncome - totalExpense

			// Gửi dữ liệu cho Gemini để viết báo cáo
			reportPrompt := fmt.Sprintf("Tổng thu: %d, Tổng chi: %d, Số dư hiện tại: %d VND. Hãy viết một đoạn nhận xét/báo cáo tài chính ngắn gọn, thân thiện và đưa ra lời khuyên.", totalIncome, totalExpense, netBalance)

			bot.Send(tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping))

			// Tạm đổi System Instruction cho báo cáo
			originalInstruction := model.SystemInstruction
			model.SystemInstruction = &genai.Content{
				Parts: []genai.Part{
					genai.Text("Bạn là một chuyên gia tư vấn tài chính cá nhân. Hãy viết báo cáo ngắn gọn, thân thiện bằng tiếng Việt."),
				},
			}
			model.ResponseMIMEType = "text/plain"

			resp, err := model.GenerateContent(ctx, genai.Text(reportPrompt))

			// Khôi phục Instruction
			model.SystemInstruction = originalInstruction
			model.ResponseMIMEType = "application/json"

			var aiInsight string
			if err != nil {
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Quota Exceeded") || strings.Contains(err.Error(), "quota") {
					aiInsight = "(Hệ thống AI đang quá tải hoặc hết hạn mức trong ngày. Bạn xem tạm tổng tiền nhé!)"
				} else {
					aiInsight = "(Không thể tạo nhận xét từ AI lúc này)"
				}
			} else {
				for _, part := range resp.Candidates[0].Content.Parts {
					aiInsight += fmt.Sprintf("%v", part)
				}
			}

			replyText := fmt.Sprintf("🟢 **TỔNG THU:** %d VND\n🔴 **TỔNG CHI:** %d VND\n💰 **SỐ DƯ:** %d VND\n\n💡 **Nhận xét từ AI:**\n%s", totalIncome, totalExpense, netBalance, aiInsight)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
			continue
		}

		if update.Message.Text == "/chart" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "📊 Đang vẽ biểu đồ chi tiêu..."))

			expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi đọc dữ liệu: "+err.Error()))
				continue
			}

			chartBytes, err := tools.GeneratePieChart(expenses)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi vẽ biểu đồ: "+err.Error()))
				continue
			}

			photo := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileBytes{
				Name:  "chart.png",
				Bytes: chartBytes,
			})
			bot.Send(photo)
			continue
		}

		if update.Message.Text == "/dashboard" {
			msgText := "🌟 **Web Dashboard**\n\nBấm vào nút bên dưới để mở giao diện quản lý tài chính nâng cao trực tiếp trên Telegram!"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
			msg.ParseMode = "Markdown"
			
			// Lấy URL gốc
			appURL := os.Getenv("WEBHOOK_URL")
			if appURL == "" {
				appURL = "https://your-ngrok-url.ngrok-free.app" // Tạm thời hardcode hoặc yêu cầu dev thay ngrok URL khi test local
			}
			appURL += "/dashboard/"

			// Nút Inline mở Web App bằng trình duyệt nội bộ của Telegram
			btn := tgbotapi.InlineKeyboardButton{
				Text: "🚀 Mở Dashboard",
				URL:  &appURL,
			}
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
			
			bot.Send(msg)
			continue
		}

		if update.Message.Text == "/undo" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Đang tiến hành xóa giao dịch cuối cùng..."))

			err := tools.UndoLastExpense(userSpreadsheet)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi hoàn tác: "+err.Error()))
			} else {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Đã xóa giao dịch cuối cùng thành công!"))
			}
			continue
		}

		if update.Message.Text == "/edit" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Đang tải danh sách 5 giao dịch gần nhất..."))
			
			recentExp, err := tools.FetchRecentExpenses(userSpreadsheet, 5)
			if err != nil || len(recentExp) == 0 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Không thể lấy danh sách giao dịch."))
				continue
			}

			msgText := "Chọn giao dịch bạn muốn sửa:\n"
			var rows [][]tgbotapi.InlineKeyboardButton
			
			for i, r := range recentExp {
				msgText += fmt.Sprintf("%d. %s: %d VND (%s)\n", i+1, r.Expense.Description, r.Expense.Amount, r.Expense.Date)
				
				btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Sửa #%d", i+1), fmt.Sprintf("edit_%d", r.RowIndex))
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
			bot.Send(msg)
			continue
		}

		if update.Message.Text == "/export" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Đang tạo file báo cáo (CSV)..."))

			expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi đọc dữ liệu: "+err.Error()))
				continue
			}

			csvBytes, err := tools.GenerateCSVReport(expenses)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi tạo file CSV: "+err.Error()))
				continue
			}

			doc := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FileBytes{
				Name:  "finance_report.csv",
				Bytes: csvBytes,
			})
			bot.Send(doc)
			continue
		}

		if strings.HasPrefix(update.Message.Text, "/ask") {
			question := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/ask"))
			if question == "" {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Vui lòng nhập câu hỏi sau lệnh /ask.\nVí dụ: `/ask Tháng trước tôi tiêu bao nhiêu tiền ăn?`"))
				continue
			}

			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🤔 Đang tra cứu dữ liệu và suy nghĩ..."))
			bot.Send(tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping))

			expenses, err := tools.FetchExpensesFromSheet(userSpreadsheet)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi đọc dữ liệu: "+err.Error()))
				continue
			}

			csvBytes, err := tools.GenerateCSVReport(expenses)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi xử lý dữ liệu: "+err.Error()))
				continue
			}

			// Gửi Prompt kèm theo dữ liệu cho Gemini
			askPrompt := fmt.Sprintf("Dưới đây là dữ liệu tài chính cá nhân của tôi định dạng CSV.\n\nCâu hỏi: %s\n\nDữ liệu CSV:\n%s", question, string(csvBytes))

			// Tạm đổi System Instruction
			originalInstruction := model.SystemInstruction
			model.SystemInstruction = &genai.Content{
				Parts: []genai.Part{
					genai.Text("Bạn là một chuyên gia kế toán. Hãy phân tích bảng dữ liệu CSV được cung cấp và trả lời câu hỏi của người dùng. Trả lời cực kỳ ngắn gọn, chính xác bằng tiếng Việt. Nếu dữ liệu không có, hãy báo không tìm thấy."),
				},
			}
			model.ResponseMIMEType = "text/plain"

			resp, err := model.GenerateContent(ctx, genai.Text(askPrompt))

			// Khôi phục Instruction
			model.SystemInstruction = originalInstruction
			model.ResponseMIMEType = "application/json"

			var replyText string
			if err != nil {
				replyText = "❌ Lỗi khi AI phân tích: " + err.Error()
			} else {
				for _, part := range resp.Candidates[0].Content.Parts {
					replyText += fmt.Sprintf("%v", part)
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🤖 **AI Trả lời:**\n"+replyText)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
			continue
		}

		log.Printf("Chat ID của bạn là: %d", update.Message.Chat.ID)

		var promptParts []genai.Part

		userText := update.Message.Text
		if update.Message.Caption != "" {
			userText = update.Message.Caption
		}

		if len(update.Message.Photo) > 0 {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "📸 Đang phân tích hóa đơn..."))

			photo := update.Message.Photo[len(update.Message.Photo)-1]
			fileURL, err := bot.GetFileDirectURL(photo.FileID)
			if err == nil {
				imgResp, errDl := http.Get(fileURL)
				if errDl == nil {
					defer imgResp.Body.Close()
					imgBytes, _ := io.ReadAll(imgResp.Body)
					promptParts = append(promptParts, genai.ImageData("jpeg", imgBytes))
				}
			}

			if userText == "" {
				userText = "Hãy trích xuất thông tin chi tiêu từ hóa đơn/ảnh chụp màn hình này."
			}
		} else if userText == "" {
			continue
		}

		msgAction := tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping)
		bot.Send(msgAction)

		log.Printf("[User %s]: %s (Có ảnh: %v)", update.Message.From.UserName, userText, len(update.Message.Photo) > 0)

		promptParts = append(promptParts, genai.Text(userText))
		resp, err := model.GenerateContent(ctx, promptParts...)

		var replyText string
		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Quota Exceeded") || strings.Contains(err.Error(), "quota") {
				replyText = "Hệ thống AI đang quá tải hoặc hết hạn mức trong ngày. Bạn vui lòng thử lại sau nhé! 🙏"
			} else {
				replyText = "❌ Lỗi khi phân tích dữ liệu: " + err.Error()
			}
		} else {
			for _, part := range resp.Candidates[0].Content.Parts {
				replyText = fmt.Sprintf("%v", part)
			}

			replyText = strings.TrimPrefix(replyText, "```json\n")
			replyText = strings.TrimSuffix(replyText, "\n```")

			exps, errParse := tools.ParseExpensesJSON(replyText)
			if errParse == nil {
				chatID := update.Message.Chat.ID
				isSuccess := false
				
				// Nếu đang sửa giao dịch (chỉ hỗ trợ 1 dòng)
				if rowIndex, ok := editState[chatID]; ok && rowIndex > 0 {
					if len(exps) > 0 {
						exp := exps[0] // Chỉ lấy phần tử đầu tiên nếu edit
						errSheet := tools.UpdateExpenseRow(userSpreadsheet, rowIndex, &exp)
						if errSheet != nil {
							replyText = "Lỗi cập nhật Database: " + errSheet.Error()
						} else {
							replyText = fmt.Sprintf("✏️ Đã SỬA thành công dòng %d: [%s] %s - %d VND", rowIndex, exp.Type, exp.Description, exp.Amount)
							delete(editState, chatID) // Xóa state
							isSuccess = true
						}
					}
				} else {
					// Ghi mới (hỗ trợ nhiều dòng)
					errSheet := tools.AppendExpensesToSheet(userSpreadsheet, exps)
					if errSheet != nil {
						replyText = "Lỗi ghi Database: " + errSheet.Error()
					} else {
						replyText = "✅ Đã ghi vào sổ:\n"
						totalAdded := 0
						for _, exp := range exps {
							replyText += fmt.Sprintf("- [%s] %s - %d VND (%s)\n", exp.Type, exp.Description, exp.Amount, exp.Category)
							totalAdded += exp.Amount
						}
						replyText += fmt.Sprintf("\n💰 Tổng cộng: %d VND", totalAdded)
						isSuccess = true
					}
				}

				// Tính năng Cảnh báo Ngân sách (Budget Alerts)
				if isSuccess {
					budgetStr := os.Getenv("MONTHLY_BUDGET")
					if budgetStr != "" {
						if budget, errB := strconv.Atoi(budgetStr); errB == nil && budget > 0 {
							allExps, errF := tools.FetchExpensesFromSheet(userSpreadsheet)
							if errF == nil {
								currentMonth := time.Now().Format("2006-01") // Lấy chuỗi YYYY-MM
								totalMonthExpense := 0
								
								for _, e := range allExps {
									if (e.Type == "Chi" || e.Type == "chi" || e.Type == "") && strings.HasPrefix(e.Date, currentMonth) {
										totalMonthExpense += e.Amount
									}
								}
								
								percent := float64(totalMonthExpense) / float64(budget) * 100
								if percent >= 100 {
									replyText += fmt.Sprintf("\n\n🚨 **BÁO ĐỘNG ĐỎ:** Bạn đã tiêu %d VND, vượt quá 100%% ngân sách tháng (%d VND)!", totalMonthExpense, budget)
								} else if percent >= 90 {
									replyText += fmt.Sprintf("\n\n⚠️ **CẢNH BÁO:** Bạn đã tiêu %d VND (%.1f%% ngân sách). Phải cực kỳ thắt lưng buộc bụng nhé!", totalMonthExpense, percent)
								} else if percent >= 80 {
									replyText += fmt.Sprintf("\n\n⚠️ **Chú ý:** Bạn đã tiêu %.1f%% ngân sách tháng. Bắt đầu hãm phanh thôi!", percent)
								}
							}
						}
					}
				}

			} else {
				replyText = "Lỗi đọc JSON: " + errParse.Error()
			}
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText)
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	}
}
