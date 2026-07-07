package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/NAMHAIIT2HUST/Finance-Agent/internal/tools"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Lỗi: Không tìm thấy file .env")
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
Người dùng sẽ nói về các khoản chi tiêu.
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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	fmt.Println("🚀 Agent đang chờ tin nhắn trên Telegram...")

	for update := range updates {
		if update.Message == nil || update.Message.Text == "" {
			continue
		}

		userText := update.Message.Text
		log.Printf("[User %s]: %s", update.Message.From.UserName, userText)

		msgAction := tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping)
		bot.Send(msgAction)

		prompt := genai.Text(userText)
		resp, err := model.GenerateContent(ctx, prompt)

		var replyText string
		if err != nil {
			replyText = "❌ Lỗi khi phân tích dữ liệu: " + err.Error()
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
