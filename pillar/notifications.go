package pillar

import (
	"time"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2"
	"fmt"
	"log"
)

type Notification struct {
	Id       bson.ObjectId `bson:"_id"`
	Created  time.Time     `bson:"_created" json:"_created"`
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
}

type Node struct {
	NodeType string        `bson:"node_type"`
	Parent   bson.ObjectId `bson:"parent"`
	User     bson.ObjectId `bson:"user"`
}

type User struct {
	Id       bson.ObjectId `bson:"_id"`
	FullName string `bson:"full_name"`  // XXX: Python used username instead. Why?
}

type Subscription struct {
	Id       bson.ObjectId `bson:"_id"`
	Notifications map[string]bool `bson:"notifications"`
}

type JsonNotification struct {
	Id                bson.ObjectId `json:"_id"`
	Actor             interface{} `json:"actor"`
	Action            interface{} `json:"action"`
	ObjectType        string `json:"object_type"`
	ObjectName        string `json:"object_name"`
	ObjectId          bson.ObjectId `json:"object_id"`
	ContextObjectType string `json:"context_object_type"`
	ContextObjectName string `json:"context_object_name"`
	ContextObjectId   bson.ObjectId `json:"context_object_id"`
	Date              time.Time `json:"date"`
	IsRead            bool `json:"is_read"`
	IsSubscribed      bool `json:"is_subscribed"`
	Subscription      bson.ObjectId `json:"subscription"`
}

type ParsedActor struct {
	UserName string `json:"username"`
	Avatar   string `json:"avatar"`
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
				"_id": 1,
				"_created": 1,
				"activity": 1,
				"is_read": 1,
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

		close(ch)
	}()

	return ch
}

func ParseNotification(notif *Notification, session *mgo.Session) (JsonNotification, bool) {
	db := session.DB(DATABASE)
	activities_collection := db.C("activities")
	activities_subscriptions_collection := db.C("activities-subscriptions")
	nodes_collection := db.C("nodes")
	users_collection := db.C("users")

	// Find the activity
	var activity Activity;
	if err := activities_collection.FindId(notif.Activity).One(&activity); err != nil {
		log.Println("Unable to find activity", notif.Activity)
		return JsonNotification{}, false
	}
	if activity.ObjectType != "node" {
		log.Println("Unsupported object type", activity.ObjectType)
		return JsonNotification{}, false
	}

	// Find the node the activity links to
	var node Node;
	if err := nodes_collection.FindId(activity.Object).One(&node); err != nil {
		log.Printf("Unable to find node %v: %v\n", activity.Object, err)
		return JsonNotification{}, false
	}
	// Initial support only for node_type comments
	if node.NodeType != "comment" {
		log.Printf("Node type '%v' not supported\n", node.NodeType)
		return JsonNotification{}, false
	}
	// Find parent node.
	var parent_node Node;
	if err := nodes_collection.FindId(node.Parent).One(&parent_node); err != nil {
		log.Printf("Unable to find parent node %v: %v\n", node.Parent, err)
		return JsonNotification{}, false
	}
	// Name the relation.
	var context_object_name string;
	if parent_node.User == notif.User {
		context_object_name = fmt.Sprintf("your %v", parent_node.NodeType)
	} else {
		var parent_comment_user User;
		err := users_collection.FindId(parent_node.User).One(&parent_comment_user)

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
	switch(activity.Verb) {
	case "replied":
		action = "replied to"
	case "commented":
		action = "left a comment on"
	default:
		action = activity.Verb
	}

	// Find out whether the user is subscribed.
	lookup := bson.M{
		"user": notif.User,
		"context_object_type": "node",
		"context_object": activity.ContextObject,
	}
	var subscription Subscription
	var is_subscribed bool
	if err := activities_subscriptions_collection.Find(lookup).One(&subscription); err != nil {
		is_subscribed = false
		log.Println("Unable to find subscription for lookup", lookup, ":", err)
	} else {
		is_subscribed = subscription.Notifications["web"]
	}

	// Parse user_actor
	var actor User
	var parsed_actor ParsedActor
	if err := users_collection.FindId(activity.ActorUser).One(&actor); err != nil {
		log.Printf("Unable to find activity.ActorUser %v: %v\n", activity.ActorUser, err)
	} else {
		parsed_actor = ParsedActor{
			UserName: actor.FullName,
			Avatar: "",  // TODO: use gravatar
		}
	}

	return JsonNotification{
		Id: notif.Id,
		Actor: parsed_actor,
		Action: action,
		ObjectType: "comment",
		ObjectName: "",
		ObjectId: activity.Object,
		ContextObjectName: context_object_name,
		ContextObjectType: parent_node.NodeType,
		ContextObjectId: activity.ContextObject,
		IsRead: notif.IsRead,
		IsSubscribed: is_subscribed,
		Subscription: subscription.Id,
	}, true
}
