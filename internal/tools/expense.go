package tools

import (
	"encoding/json"
	"fmt"
)

type Expense struct {
	Date        string `json:"date"`
	Amount      int    `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

func ParseExpense(jsonData string) {
	var exp Expense
	err := json.Unmarshal([]byte(jsonData), &exp)
	if err != nil {
		fmt.Println("❌ Lỗi bóc tách JSON:", err)
		return
	}

	fmt.Println("✅ Đã bóc tách thành công!")
	fmt.Printf("- Danh mục: %s\n- Số tiền: %d VND\n- Mô tả: %s\n", exp.Category, exp.Amount, exp.Description)
}
