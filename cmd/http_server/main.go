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

func main() {
	db, err := sql.Open("sqlite3", "./orders.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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

	_, err = db.Exec(createTable)
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

	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
