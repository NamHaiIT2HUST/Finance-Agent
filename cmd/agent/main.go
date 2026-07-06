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

	fmt.Println("🚀 Server đang khởi động...")
	fmt.Println("🧠 Đang test kết nối tới Bộ não (LLM)...")

	prompt := genai.Text("Xin chào, hãy trả lời ngắn gọn trong 1 câu: Bạn là ai?")
	resp, err := model.GenerateContent(ctx, prompt)
	if err != nil {
		log.Fatalf("Lỗi khi gọi AI: %v", err)
	}

	fmt.Println("=====================================")
	for _, part := range resp.Candidates[0].Content.Parts {
		fmt.Printf("🤖 AI Trả lời: %v\n", part)
	}
	fmt.Println("=====================================")
}
