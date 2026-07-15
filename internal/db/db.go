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
	err = DB.AutoMigrate(&User{}, &Expense{}, &Message{})
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
func SaveMessage(userID int, isUser bool, text string) error {
	msg := Message{
		UserID: userID,
		IsUser: isUser,
		Text:   text,
	}
	return DB.Create(&msg).Error
}

func GetMessagesByUser(userID int) ([]Message, error) {
	var messages []Message
	err := DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&messages).Error
	return messages, err
}
