// Package store is the MongoDB persistence layer: users, polls and the
// durable vote log. MongoDB is the source of truth; Redis is rebuilt from it.
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

type Store struct {
	client *mongo.Client
	users  *mongo.Collection
	polls  *mongo.Collection
	votes  *mongo.Collection
}

func Connect(ctx context.Context, uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(10 * time.Second).
		SetMaxPoolSize(50))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	db := client.Database(dbName)
	s := &Store{
		client: client,
		users:  db.Collection("users"),
		polls:  db.Collection("polls"),
		votes:  db.Collection("votes"),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	unique := options.Index().SetUnique(true)
	specs := []struct {
		coll  *mongo.Collection
		model mongo.IndexModel
	}{
		{s.users, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: unique}},
		{s.polls, mongo.IndexModel{Keys: bson.D{{Key: "slug", Value: 1}}, Options: unique}},
		{s.polls, mongo.IndexModel{Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}}}},
		// One ballot per voter per poll, enforced by the database itself.
		{s.votes, mongo.IndexModel{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "voterKey", Value: 1}}, Options: unique}},
	}
	for _, sp := range specs {
		if _, err := sp.coll.Indexes().CreateOne(ctx, sp.model); err != nil {
			return fmt.Errorf("create index on %s: %w", sp.coll.Name(), err)
		}
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx, readpref.Primary()) }

func (s *Store) Close(ctx context.Context) error { return s.client.Disconnect(ctx) }

// randomID returns n characters from an unambiguous lowercase alphabet.
func randomID(n int) string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789" // 32 chars: no 0/o, 1/l
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func wrapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return ErrDuplicate
	}
	return err
}
