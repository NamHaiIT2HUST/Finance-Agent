package tools

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"

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

	// Đọc dữ liệu từ cột A đến E
	readRange := "Sheet1!A:E"
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

		// Hỗ trợ định dạng cũ (4 cột) và mới (5 cột)
		var exp Expense
		if len(row) >= 5 {
			amountStr := fmt.Sprintf("%v", row[2])
			var amount int
			fmt.Sscanf(amountStr, "%d", &amount)

			exp = Expense{
				Date:        fmt.Sprintf("%v", row[0]),
				Type:        fmt.Sprintf("%v", row[1]),
				Amount:      amount,
				Category:    fmt.Sprintf("%v", row[3]),
				Description: fmt.Sprintf("%v", row[4]),
			}
		} else if len(row) >= 4 { // Legacy format
			amountStr := fmt.Sprintf("%v", row[1])
			var amount int
			fmt.Sscanf(amountStr, "%d", &amount)

			exp = Expense{
				Date:        fmt.Sprintf("%v", row[0]),
				Type:        "Chi", // Mặc định cũ là chi
				Amount:      amount,
				Category:    fmt.Sprintf("%v", row[2]),
				Description: fmt.Sprintf("%v", row[3]),
			}
		} else {
			continue
		}

		expenses = append(expenses, exp)
	}

	return expenses, nil
}

// GenerateCSVReport tạo nội dung CSV từ danh sách Expense
func GenerateCSVReport(expenses []Expense) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Ghi BOM để Excel đọc đúng tiếng Việt (UTF-8)
	buf.WriteString("\xef\xbb\xbf")

	// Ghi dòng tiêu đề
	if err := writer.Write([]string{"Ngày", "Loại", "Số Tiền", "Danh Mục", "Mô Tả"}); err != nil {
		return nil, err
	}

	// Ghi dữ liệu
	for _, exp := range expenses {
		row := []string{
			exp.Date,
			exp.Type,
			fmt.Sprintf("%d", exp.Amount),
			exp.Category,
			exp.Description,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
