package pillar

import (
	"time"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2"
	"fmt"
)

type Notification struct {
	Created  time.Time     `bson:"_created" json:"_created"`
	Activity bson.ObjectId `bson:"activity"`
}

func ForwardNotifications(user bson.ObjectId, session *mgo.Session) chan *Notification {
	ch := make(chan *Notification)

	go func() {
		my_sess := session.Copy()
		defer my_sess.Close()

		notifications := my_sess.DB(DATABASE).C("notifications")
		result := Notification{}
		var last_seen time.Time
		var query bson.M
		var selector bson.M

		for {
			// Fetch notifications from MongoDB.
			query = bson.M{
				"_created": bson.M{"$gt": last_seen},
				"user": user,
			}
			selector = bson.M{
				"_created": 1,
				"activity": 1,
			}
			iter := notifications.Find(query).Select(selector).Sort("_created").Iter()

			// Send notifications to the client.
			for iter.Next(&result) {
				last_seen = result.Created
				ch <- &result
			}

			if err := iter.Close(); err != nil {
				fmt.Println("Error fetching notifications", err)
				close(ch)
				return
			}

			time.Sleep(5 * time.Second)
		}

		close(ch)
	}()

	return ch
}
