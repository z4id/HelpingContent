package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

type TodoStore interface {
	GetAll() ([]*Todo, error)
	GetByID(int) (*Todo, error)
	Create(string) (*Todo, error)
	Update(*Todo) error
	Delete(int) error
}

type DB struct {
	*sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) EnsureMigration() error {
	_, err := db.Exec(`
  CREATE TABLE IF NOT EXISTS todos (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   title TEXT NOT NULL,
   completed BOOLEAN NOT NULL DEFAULT false,
   created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );
 `)
	return err
}

type TodoSQLStore struct {
	DB *DB
}

func (store *TodoSQLStore) GetAll() ([]*Todo, error) {
	rows, err := store.DB.Query("SELECT id, title, completed, created_at FROM todos")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []*Todo
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.CreatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, &todo)
	}
	return todos, nil
}

func (store *TodoSQLStore) GetByID(id int) (*Todo, error) {
	row := store.DB.QueryRow("SELECT id, title, completed, created_at FROM todos WHERE id = ?", id)

	var todo Todo
	if err := row.Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.CreatedAt); err != nil {
		return nil, err
	}
	return &todo, nil
}

func (store *TodoSQLStore) Create(title string) (*Todo, error) {
	res, err := store.DB.Exec("INSERT INTO todos (title) VALUES (?)", title)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return store.GetByID(int(id))
}

func (store *TodoSQLStore) Update(todo *Todo) error {
	_, err := store.DB.Exec("UPDATE todos SET title = ?, completed = ? WHERE id = ?", todo.Title, todo.Completed, todo.ID)
	return err
}

func (store *TodoSQLStore) Delete(id int) error {
	_, err := store.DB.Exec("DELETE FROM todos WHERE id = ?", id)
	return err
}

func main() {

	db, err := NewDB("todos.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.EnsureMigration(); err != nil {
		log.Fatal(err)
	}

	store := &TodoSQLStore{db}

	http.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {

		case http.MethodGet:
			todos, err := store.GetAll()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode(todos); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case http.MethodPost:
			var todo *Todo
			if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			todo, err := store.Create(todo.Title)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode(todo); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Path[len("/todos/"):])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch r.Method {

		case http.MethodGet:
			todo, err := store.GetByID(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode(todo); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case http.MethodPut:
			var todo Todo
			if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			todo.ID = id
			if err := store.Update(&todo); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode(todo); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case http.MethodDelete:
			if err := store.Delete(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
