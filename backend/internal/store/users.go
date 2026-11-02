package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"livepoll/internal/models"
)

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	u.ID = bson.NewObjectID()
	u.CreatedAt = time.Now().UTC()
	_, err := s.users.InsertOne(ctx, u)
	return wrapErr(err)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := s.users.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	return &u, wrapErr(err)
}

func (s *Store) UserByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	var u models.User
	err := s.users.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	return &u, wrapErr(err)
}
