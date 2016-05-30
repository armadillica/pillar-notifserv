package main

import (
	"encoding/json"
	"fmt"
	"github.com/armadillica/pillar-notifserv/pillar"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/mgo.v2"
	"html/template"
	"log"
	"net/http"
	"path"
)

var session *mgo.Session

func http_unauthorized(w http.ResponseWriter, err error) {
	log.Println(err.Error())
	w.Header().Add("WWW-Authenticate", "Basic")
	http.Error(w, "Cannot authenticate user", http.StatusUnauthorized)
}

func http_sse(w http.ResponseWriter, r *http.Request) {
	// Make sure that we only serve /, and not a sub-resource.
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

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

	mongo_sess := session.Copy()
	defer mongo_sess.Close()

	user, err := pillar.AuthRequest(r, session)
	if err != nil {
		http_unauthorized(w, err)
		return
	}

	notifications := pillar.ForwardNotifications(user, session)

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
			json_notif, ok = pillar.ParseNotification(n, session)
			if !ok {
				log.Println(r.RemoteAddr, "Unable to parse notification.")
				continue
			}

			msg, err := json.Marshal(json_notif)
			if err != nil {
				log.Println(r.RemoteAddr, "Unable to marshal notification as JSON:", err)
				continue
			}

			fmt.Fprintf(w, "id: %v\n", json_notif.Id)
			fmt.Fprintf(w, "event: notification\n")
			fmt.Fprintf(w, "data: %s\n\n", msg)
			f.Flush()
		}
	}

	// Done.
	log.Println(r.RemoteAddr, "Finished HTTP request at", r.URL.Path)
}

func http_template(w http.ResponseWriter, r *http.Request) {
	mongo_sess := session.Clone()
	defer mongo_sess.Close()

	// Authenticate the user.
	_, err := pillar.AuthRequest(r, session)
	if err != nil {
		http_unauthorized(w, err)
		return
	}

	// Read in the template with our SSE JavaScript code.
	template_path := path.Base(path.Clean(r.URL.Path))
	t, err := template.ParseFiles(fmt.Sprintf("templates/%s.html", template_path))
	if err != nil {
		log.Fatal("dude, error parsing your template:", err)
	}

	// Render the template, writing to `w`.
	t.Execute(w, pillar.Conf.Origin)

	// Done.
	log.Println(r.RemoteAddr, "Finished HTTP request at", r.URL.Path)
}

func main() {
	envconfig.Process("PILLAR_NOTIFSERV", &pillar.Conf)
	log.Println("MongoDB database server:", pillar.Conf.DatabaseHost)
	log.Println("MongoDB database name  :", pillar.Conf.DatabaseName)

	// Connect to MongoDB
	var err error
	session, err = mgo.Dial(pillar.Conf.DatabaseHost)
	if err != nil {
		panic(err)
	}
	session.SetMode(mgo.Monotonic, true) // Optional. Switch the session to a monotonic behavior.

	http.Handle("/", http.HandlerFunc(http_sse))

	if pillar.Conf.Origin == "" {
		log.Println("Origin not configured, /iframe/ handler not available.")
	} else {
		log.Println("Accepting embedding by : ", pillar.Conf.Origin)
		http.Handle("/iframe/", http.HandlerFunc(http_template))
	}

	log.Println("Listening at           :", pillar.Conf.Listen)

	// Fall back to insecure server if TLS certificate/key is not defined.
	if pillar.Conf.TLSCert == "" && pillar.Conf.TLSKey == "" {
		log.Println("WARNING: TLS not enabled!")
		log.Fatal(http.ListenAndServe(pillar.Conf.Listen, nil))
	}

	log.Fatal(http.ListenAndServeTLS(pillar.Conf.Listen,
		pillar.Conf.TLSCert,
		pillar.Conf.TLSKey,
		nil))
}
