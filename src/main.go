package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

var host string
var port int

const database_file string = "main.db"
const salt = "oogha3AiH7taimohreeH8Lexoonea5zi"

var (
	globalState *State
)

func initFlags() {
	flag.StringVar(&host, "host", "127.0.0.1", "the host to listen on")
	flag.StringVar(&host, "h", "127.0.0.1", "the host to listen on (shorthand)")
	flag.IntVar(&port, "port", 8080, "the port to listen on")
	flag.IntVar(&port, "p", 8080, "the port to listen on (shorthand)")
}

func main() {
	initFlags()
	flag.Parse()

	// log init
	log.Println("[i] Setting up logging...")
	logFile, err := os.OpenFile("server.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		log.Fatal("Error opening the server.log file: ", err)
	}
	logger := loggingMiddleware{logFile}

	// db init
	log.Println("[i] Setting up Global State Struct...")
	s, err := NewState()
	if err != nil {
		log.Fatal("Error creating the NewState(): ", err)
	}
	globalState = s

	// session init
	log.Println("[i] Setting up Session Storage...")
	store, err := NewSqliteStore("./sessions.db", "sessions", "/", 3600, []byte(os.Getenv("SESSION_KEY")))
	if err != nil {
		panic(err)
	}
	globalState.sessions = store

	// HTTP init
	log.Println("[i] Setting up HTTP Routes...")
	r := mux.NewRouter()
	r.Use(logger.Middleware)

	// unauthenticated endpoints
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/register", registerHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// endpoints with auth needed
	auth_needed := r.PathPrefix("/").Subrouter()
	auth_needed.Use(authMiddleware)
	auth_needed.HandleFunc("/logout", logoutHandler)

	auth_needed.HandleFunc("/bot", botsHandler)
	auth_needed.HandleFunc("/bot/new", botNewHandler)
	auth_needed.HandleFunc("/bot/{id}", botSingleHandler)

	auth_needed.HandleFunc("/user", usersHandler)
	auth_needed.HandleFunc("/user/{id}", userHandler)
	auth_needed.HandleFunc("/user/{id}/profile", profileHandler)

	auth_needed.HandleFunc("/battle", battlesHandler)
	auth_needed.HandleFunc("/battle/new", battleNewHandler)
	auth_needed.HandleFunc("/battle/{id}", battleSingleHandler)
	auth_needed.HandleFunc("/battle/{id}/submit", battleSubmitHandler)

	log.Printf("[i] HTTP Server running on %s:%d\n", host, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r))
}
