package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	getCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "http_requests_get_total",
			Help: "Number of GET requests.",
		})

	postCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "http_requests_post_total",
			Help: "Number of POST requests.",
		})

	logger        *log.Logger
	sqliteFlag    = flag.Bool("sqlite", false, "Use SQLite database for username and access timestamp logging")
	k8sFlag       = flag.Bool("k8s", false, "Use it if you run app in k8s")
	serverlogpath string
	sqlitedbpath  string
)

func init() {
	prometheus.MustRegister(getCounter)
	prometheus.MustRegister(postCounter)
}

func initLog() {
	log.Println("Trying to create sever.log file")
	file, err := os.OpenFile(serverlogpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(file, "", log.Lmsgprefix)
	log.Println("server.log file have created")
}

func initDB() {
	log.Println("Trying to create sqlite.db")
	log.Print(sqlitedbpath)
	db, err := os.Create(sqlitedbpath)
	if err != nil {
		log.Fatal(err.Error())
	}
	db.Close()
	log.Println("sqlite.db have created")

}

func prepareDB() {
	db, _ := sql.Open("sqlite3", sqlitedbpath)
	defer db.Close()
	createUserAccessTableSQL := `CREATE TABLE users (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"username" TEXT,
		"timestamp" TEXT
	  );`
	log.Println("Trying to create users table")
	statement, err := db.Prepare(createUserAccessTableSQL)
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec()
	log.Println("users table have created")
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello Page")
	getCounter.Inc()
}

func simpleUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Invalid HTTP method")
		return
	}
	r.ParseForm()
	logger.Printf("%s: %s", r.Form.Get("name"), time.Now().Format("15:04:05 - 01.02.2006"))
	postCounter.Inc()
}

func sqliteUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.Form.Get("name")
		timestamp := time.Now().Format("15:04:05 - 01.02.2006")
		db, _ := sql.Open("sqlite3", sqlitedbpath)
		defer db.Close()
		insertUserAccessTableSQL := `INSERT INTO users(username, timestamp) VALUES (?, ?);`
		statement, err := db.Prepare(insertUserAccessTableSQL)
		if err != nil {
			log.Fatalln(err.Error())
		}
		_, err = statement.Exec(username, timestamp)
		if err != nil {
			log.Fatalln(err.Error())
		}
		postCounter.Inc()
	} else if r.Method == http.MethodGet {
		names, ok := r.URL.Query()["name"]
		if !ok || len(names[0]) < 1 {
			fmt.Printf("")
			return
		}
		name := names[0]
		db, _ := sql.Open("sqlite3", sqlitedbpath)
		defer db.Close()
		var username string
		var timestamp string
		rows, _ := db.Query("SELECT username,timestamp FROM users WHERE username=?", name)
		defer rows.Close()
		for rows.Next() {
			if err := rows.Scan(&username, &timestamp); err != nil {
				log.Print(err)
			}
			fmt.Fprintf(w, "%s: %s\n", username, timestamp)
		}
		getCounter.Inc()
	}
}

func main() {
	flag.Parse()
	if *k8sFlag {
		sqlitedbpath = "/data/sqlite.db"
		serverlogpath = "/data/server.log"
	} else {
		sqlitedbpath = "sqlite.db"
		serverlogpath = "server.log"
	}
	if *sqliteFlag {
		fmt.Printf("Starting server at port 8080 with SQLite db support\n")
		initDB()
		prepareDB()
		http.HandleFunc("/user", sqliteUserHandler)
	} else {
		fmt.Printf("Starting server at port 8080 with log file support\n")
		initLog()
		http.HandleFunc("/user", simpleUserHandler)
	}

	http.HandleFunc("/hello", helloHandler)
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Ready to accept connections")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
