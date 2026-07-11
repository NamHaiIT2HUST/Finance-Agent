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
	Type        string `json:"type"` // "Thu" hoặc "Chi"
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
			{exp.Date, exp.Type, exp.Amount, exp.Category, exp.Description},
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

// ExpenseWithRow chứa thông tin chi tiêu kèm theo số dòng để tiện chỉnh sửa
type ExpenseWithRow struct {
	RowIndex int
	Expense  Expense
}

// FetchRecentExpenses lấy n giao dịch gần nhất (thường là 5)
func FetchRecentExpenses(spreadsheetId string, limit int) ([]ExpenseWithRow, error) {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("không thể đọc file credentials.json: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return nil, fmt.Errorf("lỗi khởi tạo Sheets client: %v", err)
	}

	readRange := "Sheet1!A:E"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("không thể đọc dữ liệu: %v", err)
	}

	var results []ExpenseWithRow
	// Bỏ qua dòng tiêu đề, lặp ngược từ cuối lên
	for i := len(resp.Values) - 1; i > 0 && len(results) < limit; i-- {
		row := resp.Values[i]
		
		// Hỗ trợ cả định dạng cũ (4 cột) và mới (5 cột)
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
				Type:        "Chi", // Dữ liệu cũ mặc định là Chi
				Amount:      amount,
				Category:    fmt.Sprintf("%v", row[2]),
				Description: fmt.Sprintf("%v", row[3]),
			}
		} else {
			continue
		}

		results = append(results, ExpenseWithRow{
			RowIndex: i + 1, // Sheet API là 1-based index (A1)
			Expense:  exp,
		})
	}
	return results, nil
}

// UpdateExpenseRow cập nhật lại dữ liệu tại một dòng cụ thể
func UpdateExpenseRow(spreadsheetId string, rowIndex int, exp *Expense) error {
	ctx := context.Background()

	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return fmt.Errorf("không thể đọc file credentials.json: %v", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo Sheets client: %v", err)
	}

	updateRange := fmt.Sprintf("Sheet1!A%d:E%d", rowIndex, rowIndex)
	row := &sheets.ValueRange{
		Values: [][]interface{}{
			{exp.Date, exp.Type, exp.Amount, exp.Category, exp.Description},
		},
	}

	updateCall := srv.Spreadsheets.Values.Update(spreadsheetId, updateRange, row)
	updateCall.ValueInputOption("USER_ENTERED")

	_, err = updateCall.Do()
	if err != nil {
		return fmt.Errorf("không thể cập nhật dòng %d: %v", rowIndex, err)
	}

	return nil
}
