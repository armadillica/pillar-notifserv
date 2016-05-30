package pillar

import (
	"errors"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"time"
)

type Token struct {
	Token string        `bson:"token"`
	User  bson.ObjectId `bson:"user"`
}

func AuthUser(token string, session *mgo.Session) (bson.ObjectId, error) {
	my_sess := session.Copy()
	defer my_sess.Close()

	tokens := session.DB(Conf.DatabaseName).C("tokens")
	db_token := Token{}

	query := bson.M{
		"token":       token,
		"expire_time": bson.M{"$gt": time.Now()}}

	if err := tokens.Find(query).One(&db_token); err != nil {
		log.Println("Error fetching token:", err)
		return bson.ObjectIdHex("123456789012345678901234"), err
	}

	return db_token.User, nil
}

func AuthRequest(r *http.Request, session *mgo.Session) (bson.ObjectId, error) {
	token, _, ok := r.BasicAuth()
	if !ok {
		return bson.NewObjectId(), errors.New("No authentication header given")
	}

	user, err := AuthUser(token, session)
	if err != nil {
		return bson.NewObjectId(), errors.New("Invalid credentials")
	}

	return user, nil
}
