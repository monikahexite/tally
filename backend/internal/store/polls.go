package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"livepoll/internal/models"
)

const slugLength = 8

// CreatePoll assigns ids and a short public slug, retrying on the (unlikely)
// slug collision reported by the unique index.
func (s *Store) CreatePoll(ctx context.Context, p *models.Poll) error {
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	for i := range p.Options {
		p.Options[i].ID = randomID(6)
	}
	for attempt := 0; attempt < 5; attempt++ {
		p.ID = bson.NewObjectID()
		p.Slug = randomID(slugLength)
		_, err := s.polls.InsertOne(ctx, p)
		if err = wrapErr(err); !errors.Is(err, ErrDuplicate) {
			return err
		}
	}
	return fmt.Errorf("could not allocate a unique slug")
}

func (s *Store) PollBySlug(ctx context.Context, slug string) (*models.Poll, error) {
	var p models.Poll
	err := s.polls.FindOne(ctx, bson.M{"slug": slug}).Decode(&p)
	return &p, wrapErr(err)
}

func (s *Store) PollsByOwner(ctx context.Context, owner bson.ObjectID, limit int64) ([]models.Poll, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit)
	cur, err := s.polls.Find(ctx, bson.M{"ownerId": owner}, opts)
	if err != nil {
		return nil, err
	}
	polls := []models.Poll{}
	err = cur.All(ctx, &polls)
	return polls, err
}

// SetPollStatus updates the status. Reopening also clears an expired timer,
// otherwise the poll would immediately count as closed again.
func (s *Store) SetPollStatus(ctx context.Context, id bson.ObjectID, status string) (*models.Poll, error) {
	update := bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().UTC()}}
	if status == models.StatusOpen {
		update["$unset"] = bson.M{"closesAt": ""}
	}
	var p models.Poll
	err := s.polls.FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&p)
	return &p, wrapErr(err)
}

func (s *Store) DeletePoll(ctx context.Context, id bson.ObjectID) error {
	res, err := s.polls.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	_, err = s.votes.DeleteMany(ctx, bson.M{"pollId": id})
	return err
}
