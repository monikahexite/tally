package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"livepoll/internal/models"
)

// InsertVote persists a ballot. ErrDuplicate means this voter already voted.
func (s *Store) InsertVote(ctx context.Context, v *models.Vote) error {
	v.ID = bson.NewObjectID()
	v.CreatedAt = time.Now().UTC()
	_, err := s.votes.InsertOne(ctx, v)
	return wrapErr(err)
}

// VoteOf returns the option this voter picked, or "" if they have not voted.
func (s *Store) VoteOf(ctx context.Context, pollID bson.ObjectID, voterKey string) (string, error) {
	var v models.Vote
	err := s.votes.FindOne(ctx, bson.M{"pollId": pollID, "voterKey": voterKey},
		options.FindOne().SetProjection(bson.M{"optionId": 1})).Decode(&v)
	if err = wrapErr(err); err == ErrNotFound {
		return "", nil
	}
	return v.OptionID, err
}

// VoteState rebuilds everything Redis needs for a poll: per-option counts and
// the set of voters. Called only when Redis has no live state for the poll.
func (s *Store) VoteState(ctx context.Context, pollIDHex string) (map[string]int64, []string, error) {
	pollID, err := bson.ObjectIDFromHex(pollIDHex)
	if err != nil {
		return nil, nil, err
	}
	cur, err := s.votes.Find(ctx, bson.M{"pollId": pollID},
		options.Find().SetProjection(bson.M{"optionId": 1, "voterKey": 1, "_id": 0}))
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	counts := map[string]int64{}
	var voters []string
	for cur.Next(ctx) {
		var v models.Vote
		if err := cur.Decode(&v); err != nil {
			return nil, nil, err
		}
		counts[v.OptionID]++
		voters = append(voters, v.VoterKey)
	}
	return counts, voters, cur.Err()
}

// TotalsByPoll counts ballots for several polls in one aggregation.
func (s *Store) TotalsByPoll(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]int64, error) {
	out := make(map[bson.ObjectID]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	pipeline := bson.A{
		bson.M{"$match": bson.M{"pollId": bson.M{"$in": ids}}},
		bson.M{"$group": bson.M{"_id": "$pollId", "n": bson.M{"$sum": 1}}},
	}
	cur, err := s.votes.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID bson.ObjectID `bson:"_id"`
		N  int64         `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, nil
}
