package main

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jordan-wright/unindexed"
)

/*func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}*/

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func BasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user, pass, _ := r.BasicAuth()
		var myUser string = "admin"
		var myPass string = "87654321"

		if myUser != user || myPass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Started %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("Completed %s in %v", r.URL.Path, time.Since(start))
	})
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	//w.Write([]byte("This is HOME page\n"))
	http.ServeFile(w, r, "static/home.html")
}

func SettingsHandler(w http.ResponseWriter, r *http.Request) {
	//w.Write([]byte("This is SETTINGS page\n"))
	http.ServeFile(w, r, "static/settings.html")
}

func AboutHandler(w http.ResponseWriter, r *http.Request) {
	//w.Write([]byte("This is ABOUT page\n"))
	http.ServeFile(w, r, "static/about.html")
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, _ := upgrader.Upgrade(w, r, nil) // error ignored for sake of simplicity
	for {
		// Read message from browser
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		// Print the message to the console
		log.Printf("%s sent: %s\n", conn.RemoteAddr(), string(msg))

		// Write message back to browser
		if err = conn.WriteMessage(msgType, msg); err != nil {
			return
		}
	}
}

func ReadFileExample(filename string) (string, error) {
	b, err := ioutil.ReadFile(filename) // just pass the file name
	if err != nil {
		log.Print(err)
	}

	//log.Println(b) // print the content as 'bytes'

	str := string(b) // convert content to a 'string'
	re := regexp.MustCompile(`Name=[a-zA-Z0-9]*`)
	s := strings.Split(re.FindString(str), "=")
	if len(s) < 2 {
		return "", errors.New("no one interface is present in config file")
	}

	return s[1], nil
}

func main() {

	file, err := ReadFileExample("file.txt")
	if err != nil {
		log.Printf("error reading config file: %v", err)
	}
	log.Println(file)

	//r := mux.NewRouter().StrictSlash(false)
	r := mux.NewRouter()
	//mux := http.NewServeMux()

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Routes consist of a path and a handler function.
	r.HandleFunc("/ws", BasicAuth(WebSocketHandler))
	r.HandleFunc("/home.html", BasicAuth(HomeHandler))
	r.HandleFunc("/settings.html", BasicAuth(SettingsHandler))
	r.HandleFunc("/about.html", BasicAuth(AboutHandler))
	r.Use(loggingHandler)
	////mux.Handle("/", r)
	r.PathPrefix("/").Handler(http.FileServer(unindexed.Dir("static")))

	srv := &http.Server{
		Handler: r, // Pass our instance of gorilla/mux in.
		Addr:    "127.0.0.1:8888",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	// Run our server in a goroutine so that it doesn't block.
	log.Println("Server listening...")
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("Shutting down web server!!!")
	os.Exit(0)
}
