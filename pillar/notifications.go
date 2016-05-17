package pillar

import (
	"time"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2"
	"fmt"
)

type Notification struct {
	Id       bson.ObjectId `bson:"_id"`
	Created  time.Time     `bson:"_created"`
	Updated  time.Time     `bson:"_updated"`
	User     bson.ObjectId `bson:"user"`
	Activity bson.ObjectId `bson:"activity"`
	Etag     string        `bson:"_etag"`
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

		for {
			// Fetch notifications from MongoDB.
			query = bson.M{
				"_created": bson.M{"$gt": last_seen},
				"user": user,
			}
			iter := notifications.Find(query).Sort("_created").Iter()

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
