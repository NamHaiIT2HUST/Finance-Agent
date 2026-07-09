package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
Người dùng sẽ nói về các khoản chi tiêu hoặc gửi ảnh hóa đơn.
Nhiệm vụ: Phân tích và CHỈ trả về JSON với các key: "date" (YYYY-MM-DD), "amount" (số nguyên), "category", "description". Không giải thích gì thêm.`),
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
		chatIDStr := os.Getenv("CHAT_ID")
		if chatIDStr != "" {
			chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
			msg := tgbotapi.NewMessage(chatID, "🔔 Ting ting! Hải ơi, cuối ngày rồi, xem lại xem hôm nay có khoản chi nào quên chưa đưa cho tôi nhập sổ không?")
			bot.Send(msg)
			log.Println("⏰ Đã gửi tin nhắn nhắc nhở theo lịch!")
		}
	})

	if errCron != nil {
		log.Printf("Lỗi cài đặt lịch: %v", errCron)
	}
	c.Start()

	var updates tgbotapi.UpdatesChannel
	webhookURL := os.Getenv("WEBHOOK_URL")

	if webhookURL != "" {
		// Đăng ký Webhook với Telegram
		wh, _ := tgbotapi.NewWebhook(webhookURL + "/")
		_, errWh := bot.Request(wh)
		if errWh != nil {
			log.Fatalf("Lỗi set webhook: %v", errWh)
		}

		// Lắng nghe qua channel Webhook
		updates = bot.ListenForWebhook("/")

		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		// Khởi động HTTP Server thật trên một goroutine để không block vòng lặp xử lý tin nhắn
		go func() {
			log.Printf("🌐 Đang chạy Webhook Server ở cổng %s", port)
			if errHttp := http.ListenAndServe(":"+port, nil); errHttp != nil {
				log.Fatalf("Lỗi khởi động HTTP Server: %v", errHttp)
			}
		}()
		fmt.Printf("🚀 Agent đang chờ tin nhắn (Chế độ Webhook tại %s)...\n", webhookURL)
	} else {
		// Fallback về Long Polling nếu chạy ở local
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates = bot.GetUpdatesChan(u)
		fmt.Println("🚀 Agent đang chờ tin nhắn (Chế độ Long Polling)...")
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.Text == "/report" {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "📊 Đang tổng hợp dữ liệu chi tiêu..."))

			expenses, err := tools.FetchExpensesFromSheet(os.Getenv("SPREADSHEET_ID"))
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Lỗi khi đọc dữ liệu: "+err.Error()))
				continue
			}

			totalAmount := 0
			for _, exp := range expenses {
				totalAmount += exp.Amount
			}

			// Gửi dữ liệu cho Gemini để viết báo cáo
			reportPrompt := fmt.Sprintf("Tổng chi tiêu của tôi hiện tại là %d VND. Hãy viết một đoạn nhận xét/báo cáo tài chính ngắn gọn, thân thiện và động viên.", totalAmount)

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

			replyText := fmt.Sprintf("💰 **TỔNG CHI TIÊU:** %d VND\n\n💡 **Nhận xét từ AI:**\n%s", totalAmount, aiInsight)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText)
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

			exp, errParse := tools.ParseExpenseJSON(replyText)
			if errParse == nil {
				errSheet := tools.AppendExpenseToSheet(os.Getenv("SPREADSHEET_ID"), exp)
				if errSheet != nil {
					replyText = "Lỗi ghi Database: " + errSheet.Error()
				} else {
					replyText = fmt.Sprintf("✅ Đã ghi vào sổ: %s - %d VND (%s)", exp.Description, exp.Amount, exp.Category)
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
