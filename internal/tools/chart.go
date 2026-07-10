package tools

import (
	"bytes"

	"github.com/wcharczuk/go-chart/v2"
)

// GeneratePieChart nhóm dữ liệu Expenses theo Category và trả về mảng byte ảnh PNG của Pie Chart
func GeneratePieChart(expenses []Expense) ([]byte, error) {
	// Nhóm dữ liệu theo danh mục
	categoryTotals := make(map[string]float64)
	for _, exp := range expenses {
		if exp.Category == "" {
			categoryTotals["Khác"] += float64(exp.Amount)
		} else {
			categoryTotals[exp.Category] += float64(exp.Amount)
		}
	}

	// Chuẩn bị dữ liệu cho go-chart
	var values []chart.Value
	for cat, total := range categoryTotals {
		if total > 0 {
			values = append(values, chart.Value{
				Value: total,
				Label: cat,
			})
		}
	}

	// Vẽ biểu đồ
	pie := chart.PieChart{
		Width:  512,
		Height: 512,
		Values: values,
	}

	// Xuất ra buffer
	buffer := bytes.NewBuffer(nil)
	err := pie.Render(chart.PNG, buffer)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
