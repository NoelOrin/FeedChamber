package db

import (
	"context"
	"fmt"
	"orinforum/internal/config"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoStore exposes the underlying *mongo.Database for repositories that need it.
type MongoStore interface {
	Database() *mongo.Database
	Collection(name string) *mongo.Collection
}

type mongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

func newMongoStore(ctx context.Context, cfg config.DBConfig) (Store, error) {
	uri := cfg.DSN
	if uri == "" {
		uri = fmt.Sprintf("mongodb://%s:%s",
			getEnv("MONGO_HOST", "localhost"),
			getEnv("MONGO_PORT", "27017"),
		)
	}

	dbName := getEnv("MONGO_DB", "orinforum")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &mongoStore{
		client: client,
		db:     client.Database(dbName),
	}, nil
}

func (m *mongoStore) AutoMigrate(_ ...any) error { return nil }
func (m *mongoStore) SQL() SQLStore               { return nil }
func (m *mongoStore) Mongo() MongoStore           { return m }
func (m *mongoStore) Type() BackendType           { return BackendMongo }
func (m *mongoStore) Database() *mongo.Database   { return m.db }
func (m *mongoStore) Collection(name string) *mongo.Collection {
	return m.db.Collection(name)
}

func (m *mongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}
