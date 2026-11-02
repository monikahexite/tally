// Package models holds the domain types persisted in MongoDB.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"passwordHash"`
	CreatedAt    time.Time     `bson:"createdAt"`
}

const (
	StatusOpen   = "open"
	StatusClosed = "closed"
)

type Option struct {
	ID   string `bson:"id" json:"id"`
	Text string `bson:"text" json:"text"`
}

type Poll struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Slug        string        `bson:"slug"`
	OwnerID     bson.ObjectID `bson:"ownerId"`
	OwnerName   string        `bson:"ownerName"`
	Question    string        `bson:"question"`
	Description string        `bson:"description,omitempty"`
	Options     []Option      `bson:"options"`
	Status      string        `bson:"status"`
	ClosesAt    *time.Time    `bson:"closesAt,omitempty"`
	CreatedAt   time.Time     `bson:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt"`
}

// EffectiveStatus folds the scheduled close time into the stored status.
func (p *Poll) EffectiveStatus(now time.Time) string {
	if p.Status == StatusOpen && p.ClosesAt != nil && !now.Before(*p.ClosesAt) {
		return StatusClosed
	}
	return p.Status
}

func (p *Poll) AcceptsVotes(now time.Time) bool {
	return p.EffectiveStatus(now) == StatusOpen
}

func (p *Poll) HasOption(id string) bool {
	for _, o := range p.Options {
		if o.ID == id {
			return true
		}
	}
	return false
}

func (p *Poll) OptionIDs() []string {
	ids := make([]string, len(p.Options))
	for i, o := range p.Options {
		ids[i] = o.ID
	}
	return ids
}

func (p *Poll) IsOwnedBy(userID bson.ObjectID) bool {
	return !userID.IsZero() && p.OwnerID == userID
}

// Vote is the durable record of a single ballot. (pollId, voterKey) is unique.
type Vote struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	PollID    bson.ObjectID `bson:"pollId"`
	OptionID  string        `bson:"optionId"`
	VoterKey  string        `bson:"voterKey"`
	CreatedAt time.Time     `bson:"createdAt"`
}
