CREATE TABLE books(
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    total_copies INT NOT NULL, 
    available_copies INT NOT NULL  
);


CREATE TABLE members(
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE loans(
    id SERIAL PRIMARY KEY, 
    book_id     INT NOT NULL REFERENCES books(id), 
    member_id   INT NOT NULL REFERENCES members(id),
    borrowed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at      TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ NOT NULL
);


