package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Expense struct {
	Date        string `json:"date"`
	Amount      int    `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

func ParseExpenseJSON(jsonData string) (*Expense, error) {
	var exp Expense
	err := json.Unmarshal([]byte(jsonData), &exp)
	if err != nil {
		return nil, err
	}
	return &exp, nil
}

func AppendExpenseToSheet(spreadsheetId string, exp *Expense) error {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return fmt.Errorf("không thể đọc file credentials.json: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo Sheets client: %v", err)
	}

	row := &sheets.ValueRange{
		Values: [][]interface{}{
			{exp.Date, exp.Amount, exp.Category, exp.Description},
		},
	}

	appendCall := srv.Spreadsheets.Values.Append(spreadsheetId, "Sheet1", row)
	appendCall.ValueInputOption("USER_ENTERED")

	_, err = appendCall.Do()
	if err != nil {
		return fmt.Errorf("không thể ghi vào sheet: %v", err)
	}

	log.Println("✅ Đã ghi thành công vào Database (Google Sheets)!")
	return nil
}

// UndoLastExpense xóa dòng cuối cùng (giao dịch mới nhất) trong Google Sheets
func UndoLastExpense(spreadsheetId string) error {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return fmt.Errorf("không thể đọc file credentials.json: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo Sheets client: %v", err)
	}

	// 1. Tìm số lượng dòng hiện tại bằng cách đọc cột A
	readRange := "Sheet1!A:A"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		return fmt.Errorf("không thể đọc dữ liệu để undo: %v", err)
	}

	rowCount := len(resp.Values)
	if rowCount <= 1 {
		return fmt.Errorf("không có giao dịch nào để xóa (hoặc chỉ có dòng tiêu đề)")
	}

	// 2. Tìm SheetId của "Sheet1"
	sheetResp, err := srv.Spreadsheets.Get(spreadsheetId).Do()
	if err != nil {
		return fmt.Errorf("lỗi khi lấy thông tin sheet: %v", err)
	}

	var sheetId int64 = -1
	for _, sheet := range sheetResp.Sheets {
		if sheet.Properties.Title == "Sheet1" {
			sheetId = sheet.Properties.SheetId
			break
		}
	}

	if sheetId == -1 {
		return fmt.Errorf("không tìm thấy tab Sheet1")
	}

	// 3. Xóa dòng cuối cùng (rowCount - 1 vì StartIndex là 0-based)
	// StartIndex = rowCount - 1, EndIndex = rowCount
	req := &sheets.Request{
		DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId:    sheetId,
				Dimension:  "ROWS",
				StartIndex: int64(rowCount - 1),
				EndIndex:   int64(rowCount),
			},
		},
	}

	batchReq := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetId, batchReq).Do()
	if err != nil {
		return fmt.Errorf("lỗi khi thực hiện xóa dòng: %v", err)
	}

	return nil
}
