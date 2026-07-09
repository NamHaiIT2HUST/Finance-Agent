package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// FetchExpensesFromSheet reads all rows from Sheet1 and returns a list of Expenses.
func FetchExpensesFromSheet(spreadsheetId string) ([]Expense, error) {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("không thể đọc file credentials.json: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return nil, fmt.Errorf("lỗi khởi tạo Sheets client: %v", err)
	}

	// Assuming data is in Sheet1!A:D (Date, Amount, Category, Description)
	readRange := "Sheet1!A:D"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("không thể đọc dữ liệu từ sheet: %v", err)
	}

	var expenses []Expense
	for i, row := range resp.Values {
		// Skip header if it exists. Assume header if first row has text like "Date" or "Amount"
		if i == 0 && len(row) > 1 && fmt.Sprintf("%v", row[1]) == "Amount" {
			continue
		}

		if len(row) >= 4 {
			amountStr := fmt.Sprintf("%v", row[1])
			amount, err := strconv.Atoi(amountStr)
			if err != nil {
				// If amount is not a number, maybe it's header, skip or set 0
				continue // better skip than set 0
			}

			expenses = append(expenses, Expense{
				Date:        fmt.Sprintf("%v", row[0]),
				Amount:      amount,
				Category:    fmt.Sprintf("%v", row[2]),
				Description: fmt.Sprintf("%v", row[3]),
			})
		}
	}

	return expenses, nil
}
