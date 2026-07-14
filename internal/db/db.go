package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

type Expense struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
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

// InitDB initializes the SQLite database
func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatal("Lỗi kết nối SQLite:", err)
	}

	createTables()
	seedAdmin()
}

func createTables() {
	userTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT DEFAULT 'user'
	);`

	expenseTable := `
	CREATE TABLE IF NOT EXISTS expenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		date TEXT,
		type TEXT,
		amount INTEGER,
		category TEXT,
		description TEXT,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	_, err := DB.Exec(userTable)
	if err != nil {
		log.Fatal("Lỗi tạo bảng users:", err)
	}
	_, err = DB.Exec(expenseTable)
	if err != nil {
		log.Fatal("Lỗi tạo bảng expenses:", err)
	}
}

func seedAdmin() {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		DB.Exec("INSERT INTO users (full_name, username, password_hash, role) VALUES (?, ?, ?, ?)", "Quản trị viên", "admin", string(hash), "admin")
	}
}

// User Methods
func CreateUser(fullName, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = DB.Exec("INSERT INTO users (full_name, username, password_hash, role) VALUES (?, ?, ?, ?)", fullName, username, string(hash), "user")
	return err
}

func GetUserByUsername(username string) (*User, error) {
	var user User
	var hash string
	err := DB.QueryRow("SELECT id, full_name, username, password_hash, role FROM users WHERE username = ?", username).Scan(&user.ID, &user.FullName, &user.Username, &hash, &user.Role)
	if err != nil {
		return nil, err
	}
	user.Password = hash
	return &user, nil
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Admin Methods
func GetAllUsers() ([]UserStat, error) {
	rows, err := DB.Query(`
		SELECT u.id, u.full_name, u.username, u.role, COUNT(e.id) as tx_count
		FROM users u
		LEFT JOIN expenses e ON u.id = e.user_id
		GROUP BY u.id
		ORDER BY u.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserStat
	for rows.Next() {
		var s UserStat
		if err := rows.Scan(&s.ID, &s.FullName, &s.Username, &s.Role, &s.TxCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func DeleteUser(id int) error {
	// Enable foreign key constraints in SQLite connection to cascade delete expenses
	DB.Exec("PRAGMA foreign_keys = ON")
	_, err := DB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// Expense Methods
func AddExpense(userID int, exp *Expense) error {
	_, err := DB.Exec(`INSERT INTO expenses (user_id, date, type, amount, category, description) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, exp.Date, exp.Type, exp.Amount, exp.Category, exp.Description)
	return err
}

func GetExpensesByUser(userID int) ([]Expense, error) {
	rows, err := DB.Query("SELECT id, user_id, date, type, amount, category, description FROM expenses WHERE user_id = ? ORDER BY date ASC, id ASC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Date, &e.Type, &e.Amount, &e.Category, &e.Description); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, nil
}
