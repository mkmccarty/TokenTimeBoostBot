package db

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var mongoDB *mongo.Client

// Open is used to establish the database connection
func Open() {

	/*
		err := godotenv.Load()
		if err != nil {
			log.Print(err)
		}

		var uri string

		if uri = os.Getenv("MONGODB_URI"); uri == "" {
			log.Print("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
			return
		}

		//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		//	defer cancel()

		client, err := mongo.Connect(options.Client().ApplyURI(uri))

		if err != nil {
			log.Print(err)
		} else {

			mongoDB = client
		}
	*/
}

// StoreRecord writes a single record to a collection
func StoreRecord(database string, collection string, key string, document interface{}) {
	/*
		if mongoDB == nil {
			return
		}

		// Updates the first document that has the specified "_id" value
		collection = mongoDB.Database(database).Collection(collection)
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, _ := collection.InsertOne(ctx, bson.D{{"name", "pi"}, {"value", 3.14159}})

		filter := bson.D{{Key: "hash", Value: key}}
		result, err := collection.UpdateOne(ctx, filter, document, opts)

		if err != nil {
			log.Print(err)
		}

		fmt.Printf("Document inserted with ID: %s\n", result.UpsertedID)
	*/
}

// Close is used to nicely shutdown the DB connection
func Close() {
	/*
		if err := mongoDB.Disconnect(ctx); err != nil {
			panic(err)
		}
	*/
}
