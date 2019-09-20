package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DB MongoDB
type DB struct {
	*mongo.Client
}

// NewDB created and returns DB connection to MongoDB
func NewDB(connStr string) (*DB, error) {
	dbClient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(connStr))
	if err != nil {
		// Failed to connect to db
		return nil, err
	}

	// check the connection
	err = dbClient.Ping(context.TODO(), nil)
	if err != nil {
		// Failed to ping db
		return nil, err
	}

	return &DB{dbClient}, nil
}
