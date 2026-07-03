package main

type Book struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	TotalCopies     int    `json:"total_copies"`
	AvailableCopies int    `json:"availableCopies"`
}

type BookRepository interface {
	GetAll() ([]Book, error)
	GetByID() (Book, error)
	Create(b Book) error
	DeleteByID(id int) error
}

type LoanRepository interface {
}

type MemberRepository interface {
}
