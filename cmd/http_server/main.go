package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

type Order struct {
	CustomerName    string
	ProductName     string
	Quantity        int
	ShippingAddress string
	Priority        string
}

func initDB(db *sql.DB) error {
	createTable := `
    CREATE TABLE IF NOT EXISTS orders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        customer_name TEXT NOT NULL,
        product_name TEXT NOT NULL,
        quantity INTEGER NOT NULL,
        shipping_address TEXT NOT NULL,
        priority TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`

	_, err := db.Exec(createTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS priority_changes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            order_id INTEGER NOT NULL,
            priority TEXT NOT NULL,
            processed BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(order_id) REFERENCES orders(id)
        )`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS polling_state (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            last_processed_id INTEGER NOT NULL DEFAULT 0
        )`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        INSERT OR IGNORE INTO polling_state (id, last_processed_id) 
        VALUES (1, 0)`)
	return err
}

func main() {
	db, err := sql.Open("sqlite3", "./orders.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = initDB(db)
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	http.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		stmt, err := db.Prepare(`
            INSERT INTO orders (
                customer_name, 
                product_name, 
                quantity, 
                shipping_address, 
                priority
            ) VALUES (?, ?, ?, ?, ?)
        `)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		quantity := r.FormValue("quantity")
		result, err := stmt.Exec(
			r.FormValue("customerName"),
			r.FormValue("productName"),
			quantity,
			r.FormValue("shippingAddress"),
			r.FormValue("priority"),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		lastID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf(
			"Inserted order #%d with quantity: %s, customer name: %s, product name: %s, shipping address: %s, priority: %s",
			lastID,
			quantity,
			r.FormValue("customerName"),
			r.FormValue("productName"),
			r.FormValue("shippingAddress"),
			r.FormValue("priority"),
		)

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc(
		"/orders/priority",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			err := r.ParseForm()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			orderID := r.FormValue("id")

			tx, err := db.Begin()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			updateStmt, err := tx.Prepare(`
				UPDATE orders 
				SET priority = 'high'
				WHERE id = ?
			`)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer updateStmt.Close()

			_, err = updateStmt.Exec(orderID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			insertStmt, err := tx.Prepare(`
				INSERT INTO priority_changes (order_id, priority)
				VALUES (?, 'high')
			`)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer insertStmt.Close()

			_, err = insertStmt.Exec(orderID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = tx.Commit()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			log.Printf(
				"Updated order #%s priority to high and logged change",
				orderID,
			)
			w.WriteHeader(http.StatusOK)
		},
	)

	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
