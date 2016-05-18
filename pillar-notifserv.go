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
	"log"
	"net/http"
	"github.com/armadillica/pillar-notifserv/pillar"
	"gopkg.in/mgo.v2"
	"encoding/json"
)


type SSE struct{
	session *mgo.Session
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

	var json_notif pillar.JsonNotification
	for {
		select {
		case <-close_notifier.CloseNotify():
			log.Println(r.RemoteAddr, "Connection closed.")
			return
		case n := <-notifications:
			json_notif, ok = pillar.ParseNotification(n, self.session)
			if !ok {
				log.Println(r.RemoteAddr, "Unable to parse notification.")
				continue
			}

			msg, err := json.Marshal(json_notif)
			if err != nil {
				log.Println(r.RemoteAddr, "Unable to marshal notification as JSON:", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			f.Flush()
		}
	}

	// Done.
	log.Println(r.RemoteAddr, "Finished HTTP request at", r.URL.Path)
}


func main() {
	addr := ":8000"

	// Connect to MongoDB
	session, err := mgo.Dial(pillar.DATABASE_HOST)
	if err != nil {
		panic(err)
	}
	session.SetMode(mgo.Monotonic, true) // Optional. Switch the session to a monotonic behavior.
	sse := &SSE{session}

	http.Handle("/", sse)

	log.Println("Listening at", addr)
	http.ListenAndServe(addr, nil)
}
