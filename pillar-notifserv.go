// Golang HTML5 Server Side Events Example
//
// Run this code like:
//  > go run server.go
//
// Then open up your browser to http://localhost:8000
// Your browser must support HTML5 SSE, of course.

// Source: https://github.com/kljensen/golang-html5-sse-example/blob/master/server.go

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"github.com/armadillica/pillar-notifserv/pillar"
	"gopkg.in/mgo.v2"
	"encoding/json"
	"gopkg.in/mgo.v2/bson"
)


type SSE struct{
	session *mgo.Session
}

type JsonNotification struct {
	Activity bson.ObjectId `json:"activity"`

}

func (self *SSE) convert_notification(notif *pillar.Notification) JsonNotification {
	return JsonNotification{
		Activity: notif.Activity,
	}
}

func (self *SSE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, "Channel started at", r.URL.Path)

	// Make sure that the writer supports flushing.
	//
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Listen to the closing of the http connection via the CloseNotifier
	close_notifier, ok := w.(http.CloseNotifier)
	if !ok {
		http.Error(w, "Cannot stream", http.StatusInternalServerError)
		return
	}

	// Authenticate the user.
	token, _, ok := r.BasicAuth()
	if !ok {
		log.Println("Unable to obtain user credentials.")
		http.Error(w, "Unable to obtain user credentials.", http.StatusForbidden)
		return
	}
	user, err := pillar.AuthUser(token, self.session)
	if err != nil {
		log.Println("Unable to authenticate user:", err)
		http.Error(w, "Cannot authenticate user", http.StatusForbidden)
		return
	}

	notifications := pillar.ForwardNotifications(user, self.session)

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var json_notif JsonNotification
	for {
		select {
		case <-close_notifier.CloseNotify():
			log.Println(r.RemoteAddr, "Connection closed.")
			return
		case n := <-notifications:
			json_notif = self.convert_notification(n)
			msg, err := json.Marshal(json_notif)
			if err != nil {
				log.Println(r.RemoteAddr, "Unable to marshal notification as JSON:", err)
				break
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			f.Flush()
		}
	}

	// Done.
	log.Println(r.RemoteAddr, "Finished HTTP request at", r.URL.Path)
}

// Handler for the main page, which we wire up to the
// route at "/" below in `main`.
//
func MainPageHandler(w http.ResponseWriter, r *http.Request) {

	// Did you know Golang's ServeMux matches only the
	// prefix of the request URL?  It's true.  Here we
	// insist the path is just "/".
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Read in the template with our SSE JavaScript code.
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("dude, error parsing your template:", err)
	}

	// Render the template, writing to `w`.
	t.Execute(w, "Duder")

	// Done.
	log.Println(r.RemoteAddr, "Finished HTTP request at", r.URL.Path)
}

// Main routine
//
func main() {
	addr := ":8000"

	// Connect to MongoDB
	session, err := mgo.Dial(pillar.DATABASE_HOST)
	if err != nil {
		panic(err)
	}
	session.SetMode(mgo.Monotonic, true) // Optional. Switch the session to a monotonic behavior.
	sse := &SSE{session}

	http.Handle("/events/", sse)
	http.Handle("/", http.HandlerFunc(MainPageHandler))

	log.Println("Listening at", addr)
	http.ListenAndServe(addr, nil)
}
