package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type BookRepository struct {
	DB *sql.DB // exported field
}

type Book struct {
	ID      int
	Title   string
	Author  string
	Year    int
	Created string
}

func (r *BookRepository) GetAll() ([]Book, error) {
	rows, err := r.DB.Query(`
		SELECT id, title, author, year, created
		FROM books
		ORDER BY created DESC
	`)

	if err != nil {
		return nil, fmt.Errorf("GetAll Query %w", err)
	}
	defer rows.Close()

	var books []Book

	for rows.Next() {
		var b Book

		err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.Created)

		if err != nil {
			return nil, fmt.Errorf("GetAll scan %w", err)
		}
		books = append(books, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetAll rows %w", err)
	}

	return books, nil
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil

}

func main() {
	dsn := "postgres://myuser:mypass@localhost:5432/mydatabase?sslmode=disable"

	db, err := openDB(dsn)

	if err != nil {
		fmt.Printf("ping error: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Printf("Successfully connecting database!\n")
	book_repo := BookRepository{
		DB: db,
	}

	books, err := book_repo.GetAll()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%v", books)
}
