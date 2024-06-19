package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoDB *mongo.Client

// Open is used to establish the database connection
func Open() {
	godotenv.Load()

	var uri string

	if uri = os.Getenv("MONGODB_URI"); uri == "" {
		log.Print("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
		return
	}

	//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//	defer cancel()

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))

	if err != nil {
		log.Print(err)
	} else {

		mongoDB = client
	}
}

// StoreRecord writes a single record to a collection
func StoreRecord(database string, collection string, key string, document interface{}) {
	if mongoDB == nil {
		return
	}

	// Updates the first document that has the specified "_id" value
	coll := mongoDB.Database(database).Collection(collection)
	opts := options.Update().SetUpsert(true)

	filter := bson.D{{Key: "hash", Value: key}}
	result, err := coll.UpdateOne(context.TODO(), filter, document, opts)

	if err != nil {
		log.Print(err)
	}

	fmt.Printf("Document inserted with ID: %s\n", result.UpsertedID)
}

// Close is used to nicely shutdown the DB connection
func Close() {
	if err := mongoDB.Disconnect(context.TODO()); err != nil {
		panic(err)
	}
}
