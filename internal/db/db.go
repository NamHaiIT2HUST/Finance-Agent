package db

import (
	"errors"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

type User struct {
	ID           int       `json:"id" gorm:"primaryKey"`
	FullName     string    `json:"full_name"`
	Username     string    `json:"username" gorm:"uniqueIndex"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role" gorm:"default:'user'"`
	Expenses     []Expense `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	Messages     []Message `json:"-" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
}

type Message struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	UserID    int       `json:"user_id" gorm:"index"`
	IsUser    bool      `json:"is_user"` // true for user, false for AI
	Text      string    `json:"text"`
	Status    string    `json:"status" gorm:"default:'completed'"` // pending, completed, error
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type Expense struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	UserID      int    `json:"user_id" gorm:"index"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	Amount      int    `json:"amount"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type QuotaLog struct {
	Date           string `json:"date" gorm:"primaryKey"`
	ApiRequests    int    `json:"api_requests"`
	FailedRequests int    `json:"failed_requests"`
}

// UserStat is for Admin Dashboard
type UserStat struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TxCount  int    `json:"tx_count"`
}

// InitDB initializes the database via GORM
func InitDB(filepath string) {
	var err error
	var dialector gorm.Dialector

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		log.Println("🌍 Kết nối tới PostgreSQL Cloud...")
		dialector = postgres.Open(dbURL)
	} else {
		log.Println("🏠 Kết nối tới SQLite Local...")
		dialector = sqlite.Open(filepath)
	}

	DB, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		log.Fatal("Lỗi kết nối DB:", err)
	}

	// Tự động tạo/cập nhật bảng
	err = DB.AutoMigrate(&User{}, &Expense{}, &Message{}, &QuotaLog{})
	if err != nil {
		log.Fatal("Lỗi Migrate DB:", err)
	}

	seedAdmin()
}

func seedAdmin() {
	var count int64
	DB.Model(&User{}).Where("username = ?", "admin").Count(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		DB.Create(&User{
			FullName:     "Quản trị viên",
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "admin",
		})
	}
}

// User Methods
func CreateUser(fullName, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	
	user := User{
		FullName:     fullName,
		Username:     username,
		PasswordHash: string(hash),
		Role:         "user",
	}
	result := DB.Create(&user)
	return result.Error
}

func GetUserByUsername(username string) (*User, error) {
	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func CheckPassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// Admin Methods
func GetAllUsers() ([]UserStat, error) {
	var stats []UserStat
	// GORM raw query
	rows, err := DB.Raw(`
		SELECT u.id, u.full_name, u.username, u.role, COUNT(e.id) as tx_count
		FROM users u
		LEFT JOIN expenses e ON u.id = e.user_id
		GROUP BY u.id, u.full_name, u.username, u.role
		ORDER BY u.id DESC
	`).Rows()
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s UserStat
		if err := DB.ScanRows(rows, &s); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func DeleteUser(id int) error {
	if id == 0 {
		return errors.New("invalid id")
	}
	// GORM Cascade delete if configured properly, or manual
	return DB.Select("Expenses", "Messages").Delete(&User{ID: id}).Error
}

// Expense Methods
func AddExpense(userID int, exp *Expense) error {
	exp.UserID = userID
	return DB.Create(exp).Error
}

func GetExpensesByUser(userID int) ([]Expense, error) {
	var expenses []Expense
	err := DB.Where("user_id = ?", userID).Order("date ASC, id ASC").Find(&expenses).Error
	return expenses, err
}

// Message Methods
func SaveMessage(userID int, isUser bool, text string, status string) (Message, error) {
	if status == "" {
		status = "completed"
	}
	msg := Message{
		UserID: userID,
		IsUser: isUser,
		Text:   text,
		Status: status,
	}
	err := DB.Create(&msg).Error
	return msg, err
}

func UpdateMessage(id int, text string, status string) error {
	return DB.Model(&Message{}).Where("id = ?", id).Updates(map[string]interface{}{"text": text, "status": status}).Error
}

func FailPendingMessages() {
	DB.Model(&Message{}).Where("status = ?", "pending").Updates(map[string]interface{}{
		"status": "error",
		"text":   "❌ Quá trình phân tích bị gián đoạn do máy chủ khởi động lại. Vui lòng gửi lại.",
	})
}

func GetMessageByID(id int) (Message, error) {
	var msg Message
	err := DB.First(&msg, id).Error
	return msg, err
}

func GetMessagesByUser(userID int) ([]Message, error) {
	var messages []Message
	err := DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&messages).Error
	return messages, err
}

func GetRecentExpenses(userID int, limit int) ([]Expense, error) {
	var expenses []Expense
	err := DB.Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&expenses).Error
	return expenses, err
}

// System Stats Methods
func IncrementQuotaUsage(success bool) {
	today := time.Now().Format("2006-01-02")
	var log QuotaLog
	err := DB.FirstOrCreate(&log, QuotaLog{Date: today}).Error
	if err == nil {
		if success {
			DB.Model(&log).Update("api_requests", gorm.Expr("api_requests + ?", 1))
		} else {
			DB.Model(&log).Update("failed_requests", gorm.Expr("failed_requests + ?", 1))
		}
	}
}

type SystemStatsData struct {
	TotalUsers    int64      `json:"total_users"`
	TotalMessages int64      `json:"total_messages"`
	TotalPrompts  int64      `json:"total_prompts"`
	TodayQuota    QuotaLog   `json:"today_quota"`
}

func GetSystemStats() (SystemStatsData, error) {
	var stats SystemStatsData
	today := time.Now().Format("2006-01-02")

	DB.Model(&User{}).Count(&stats.TotalUsers)
	DB.Model(&Message{}).Count(&stats.TotalMessages)
	DB.Model(&Message{}).Where("is_user = ?", true).Count(&stats.TotalPrompts)
	DB.FirstOrCreate(&stats.TodayQuota, QuotaLog{Date: today})

	return stats, nil
}
