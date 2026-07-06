package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Lỗi: Không tìm thấy file .env")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("Lỗi: Chưa cài đặt GEMINI_API_KEY trong file .env")
	}

	ctx := context.Background()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Lỗi khởi tạo client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-3.5-flash")

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`Bạn là một Agent quản lý tài chính. 
Người dùng sẽ nói cho bạn biết họ vừa tiêu gì. 
Nhiệm vụ của bạn là bóc tách thông tin và CHỈ trả về một chuỗi JSON chuẩn mực với các key: "date" (YYYY-MM-DD), "amount" (số nguyên), "category" (phân loại), "description" (mô tả ngắn). Không giải thích gì thêm.`),
		},
	}
	model.ResponseMIMEType = "application/json"

	fmt.Println("🚀 Agent Tài chính đang lắng nghe...")

	userInput := "Nay lúc 12h trưa tui đi ăn bát phở nạm bò hết 45 cành, xong mua cốc trà đá 5k nữa."
	fmt.Println("🗣️ User:", userInput)

	prompt := genai.Text(userInput)
	resp, err := model.GenerateContent(ctx, prompt)
	if err != nil {
		log.Fatalf("Lỗi khi gọi AI: %v", err)
	}

	fmt.Println("=====================================")
	for _, part := range resp.Candidates[0].Content.Parts {
		fmt.Printf("🤖 AI Output (JSON):\n%v\n", part)
	}
	fmt.Println("=====================================")
}
