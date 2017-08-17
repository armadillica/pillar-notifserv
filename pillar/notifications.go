package pillar

import (
	"fmt"
	"log"
	"time"

	"github.com/eefret/gravatar"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Notification struct {
	Id       bson.ObjectId `bson:"_id"`
	Created  time.Time     `bson:"_created"`
	Activity bson.ObjectId `bson:"activity"`
	User     bson.ObjectId `bson:"user"`
	IsRead   bool          `bson:"is_read"`
}

type Activity struct {
	Object        bson.ObjectId `bson:"object"`
	ObjectType    string        `bson:"object_type"`
	ContextObject bson.ObjectId `bson:"context_object"`
	Verb          string        `bson:"verb"`
	ActorUser     bson.ObjectId `bson:"actor_user"`
	Created       time.Time     `bson:"_created"`
}

type Node struct {
	NodeType string        `bson:"node_type"`
	Parent   bson.ObjectId `bson:"parent"`
	User     bson.ObjectId `bson:"user"`
}

type User struct {
	Id       bson.ObjectId `bson:"_id"`
	FullName string        `bson:"full_name"` // XXX: Python used username instead. Why?
	Email    string        `bson:"email"`
}

type Subscription struct {
	Id            bson.ObjectId   `bson:"_id"`
	Notifications map[string]bool `bson:"notifications"`
}

type JsonNotification struct {
	Id                bson.ObjectId `json:"_id"`
	Actor             string        `json:"username"`
	Avatar            string        `json:"username_avatar"`
	Action            interface{}   `json:"action"`
	ObjectType        string        `json:"object_type"`
	ObjectName        string        `json:"object_name"`
	ObjectURL         string        `json:"object_url"`
	ContextObjectType string        `json:"context_object_type"`
	ContextObjectName string        `json:"context_object_name"`
	ContextObjectURL  string        `json:"context_object_url"`
	Date              time.Time     `json:"date"`
	IsRead            bool          `json:"is_read"`
	IsSubscribed      bool          `json:"is_subscribed"`
	Subscription      bson.ObjectId `json:"subscription"`
}

func ForwardNotifications(user bson.ObjectId, session *mgo.Session) chan *Notification {
	ch := make(chan *Notification)

	go func() {
		my_sess := session.Copy()
		defer my_sess.Close()

		notifications := my_sess.DB(Conf.DatabaseName).C("notifications")
		result := Notification{}
		var last_seen time.Time
		var query bson.M
		var selector bson.M

		for {
			// Fetch notifications from MongoDB.
			query = bson.M{
				"_created": bson.M{"$gt": last_seen},
				"user":     user,
			}
			selector = bson.M{
				"_id":      1,
				"_created": 1,
				"activity": 1,
				"is_read":  1,
			}
			iter := notifications.Find(query).Select(selector).Sort("_created").Iter()

			// Send notifications to the client.
			for iter.Next(&result) {
				last_seen = result.Created
				result.User = user
				ch <- &result
			}

			if err := iter.Close(); err != nil {
				fmt.Println("Error fetching notifications", err)
				close(ch)
				return
			}

			time.Sleep(5 * time.Second)
		}
	}()

	return ch
}

func ParseNotification(notif *Notification, session *mgo.Session) (JsonNotification, bool) {
	db := session.DB(Conf.DatabaseName)
	activities_collection := db.C("activities")
	actsub_collection := db.C("activities-subscriptions")
	nodes_collection := db.C("nodes")
	users_collection := db.C("users")

	var selector bson.M

	// Find the activity
	var activity Activity
	selector = bson.M{
		"object_type":    1,
		"object":         1,
		"verb":           1,
		"context_object": 1,
		"actor_user":     1,
		"_created":       1}
	if err := activities_collection.FindId(notif.Activity).
		Select(selector).
		One(&activity); err != nil {
		log.Println("Unable to find activity", notif.Activity)
		return JsonNotification{}, false
	}
	if activity.ObjectType != "node" {
		log.Println("Unsupported object type", activity.ObjectType)
		return JsonNotification{}, false
	}

	// Find the node the activity links to
	var node Node
	selector = bson.M{"node_type": 1, "user": 1, "parent": 1}
	if err := nodes_collection.FindId(activity.Object).Select(selector).One(&node); err != nil {
		log.Printf("Unable to find node %v: %v\n", activity.Object, err)
		return JsonNotification{}, false
	}
	// Initial support only for node_type comments
	if node.NodeType != "comment" {
		log.Printf("Node type '%v' not supported\n", node.NodeType)
		return JsonNotification{}, false
	}
	// Find parent node.
	var parent_node Node
	selector = bson.M{"node_type": 1, "user": 1}
	if err := nodes_collection.FindId(node.Parent).Select(selector).One(&parent_node); err != nil {
		log.Printf("Unable to find parent node %v: %v\n", node.Parent, err)
		return JsonNotification{}, false
	}
	// Name the relation.
	var context_object_name string
	if parent_node.User == notif.User {
		context_object_name = fmt.Sprintf("your %v", parent_node.NodeType)
	} else {
		var parent_comment_user User
		err := users_collection.FindId(parent_node.User).
			Select(bson.M{"_id": 1, "full_name": 1}).
			One(&parent_comment_user)

		switch {
		case err != nil:
			context_object_name = "unknown"
		case parent_comment_user.Id == node.User:
			context_object_name = fmt.Sprintf("their %v", parent_node.NodeType)
		default:
			context_object_name = fmt.Sprintf("%v's %v", parent_comment_user.FullName,
				parent_node.NodeType)
		}
	}
	// Turn the verb into a description.
	var action string
	switch activity.Verb {
	case "replied":
		action = "replied to"
	case "commented":
		action = "left a comment on"
	default:
		action = activity.Verb
	}

	// Find out whether the user is subscribed.
	lookup := bson.M{
		"user":                notif.User,
		"context_object_type": "node",
		"context_object":      activity.ContextObject,
	}
	var subscription Subscription
	var is_subscribed bool
	selector = bson.M{"notifications": 1, "_id": 1}
	if err := actsub_collection.Find(lookup).Select(selector).One(&subscription); err != nil {
		is_subscribed = false
		log.Println("Unable to find subscription for lookup", lookup, ":", err)
	} else {
		is_subscribed = subscription.Notifications["web"]
	}

	// Parse user_actor
	var actor User
	selector = bson.M{"full_name": 1, "email": 1}
	if err := users_collection.FindId(activity.ActorUser).Select(selector).One(&actor); err != nil {
		log.Printf("Unable to find activity.ActorUser %v: %v\n", activity.ActorUser, err)
	}

	var grav_url string
	if grav, gerr := gravatar.New(); gerr == nil {
		grav_url = grav.URLParse(actor.Email)
	}

	return JsonNotification{
		Id:                notif.Id,
		Actor:             actor.FullName,
		Avatar:            grav_url,
		Action:            action,
		ObjectType:        "comment",
		ObjectName:        "",
		ObjectURL:         fmt.Sprintf("/nodes/%s/redir", activity.Object.Hex()),
		ContextObjectName: context_object_name,
		ContextObjectType: parent_node.NodeType,
		ContextObjectURL:  fmt.Sprintf("/nodes/%s/redir", activity.ContextObject.Hex()),
		IsRead:            notif.IsRead,
		IsSubscribed:      is_subscribed,
		Subscription:      subscription.Id,
		Date:              activity.Created,
	}, true
}
