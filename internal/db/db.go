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
	Username string `json:"username"`
	Password string `json:"-"`
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

// InitDB initializes the SQLite database
func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatal("Lỗi kết nối SQLite:", err)
	}

	createTables()
}

func createTables() {
	userTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
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
		FOREIGN KEY (user_id) REFERENCES users(id)
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

// User Methods
func CreateUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = DB.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

func GetUserByUsername(username string) (*User, error) {
	var user User
	var hash string
	err := DB.QueryRow("SELECT id, username, password_hash FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &hash)
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
