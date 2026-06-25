CREATE TABLE IF NOT EXISTS books (
    id      SERIAL PRIMARY KEY,
    title   TEXT NOT NULL,
    author  TEXT NOT NULL,
    year    INT  NOT NULL,
    created TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO books (title, author, year) VALUES
    ('The Go Programming Language', 'Donovan & Kernighan', 2015),
    ('Clean Code', 'Robert Martin', 2008),
    ('Designing Data-Intensive Applications', 'Martin Kleppmann', 2017);