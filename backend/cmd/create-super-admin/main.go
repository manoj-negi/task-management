package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Admin setup failed:", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is not set; export it in this terminal before running this command")
	}

	reader := bufio.NewReader(os.Stdin)
	name, err := prompt(reader, "Full name: ")
	if err != nil {
		return err
	}
	email, err := prompt(reader, "Email: ")
	if err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || !strings.Contains(email, "@") {
		return errors.New("full name and a valid email address are required")
	}

	fmt.Print("Password (input hidden): ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	password := string(passwordBytes)
	for i := range passwordBytes {
		passwordBytes[i] = 0
	}
	if len(password) < 12 {
		return errors.New("use a password of at least 12 characters")
	}
	confirmBytes, err := readPassword("Confirm password (input hidden): ")
	if err != nil {
		return err
	}
	confirm := string(confirmBytes)
	for i := range confirmBytes {
		confirmBytes[i] = 0
	}
	if password != confirm {
		return errors.New("passwords do not match")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	password = ""
	confirm = ""
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Serialize bootstrap attempts so two simultaneous runs cannot both pass the check.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock users table: %w", err)
	}

	var existing int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users u JOIN roles r ON r.id=u.role_id WHERE r.name = 'super_admin'`,
	).Scan(&existing); err != nil {
		return fmt.Errorf("check existing Super Admin: %w", err)
	}
	if existing > 0 {
		return errors.New("a Super Admin already exists; refusing to create another")
	}

	var id, savedName, savedEmail, role string
	var canAssign bool
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (full_name, email, password_hash, role_id, is_active)
		SELECT $1, $2, $3, id, TRUE FROM roles WHERE name = 'super_admin'
		RETURNING id::text, full_name, email, 'super_admin', TRUE
	`, strings.TrimSpace(name), email, string(hash)).Scan(
		&id, &savedName, &savedEmail, &role, &canAssign,
	)
	if err != nil {
		return fmt.Errorf("create Super Admin (email may already exist): %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	fmt.Println("\nSuper Admin created successfully.")
	fmt.Printf("ID: %s\nName: %s\nEmail: %s\nRole: %s\nCan assign tasks: %t\n",
		id, savedName, savedEmail, role, canAssign)
	fmt.Println("Keep your password private. This command does not print or store it in the source code.")
	return nil
}

func prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func readPassword(label string) ([]byte, error) {
	fmt.Print(label)
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return value, nil
}
