package datastore

import (
	"time"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/queries"
)

type PwHashType string

const (
	PhtArgon2id PwHashType = PwHashType(queries.PwHashTypeTArgon2id)
)

// We shouldn't be getting invalid hash type
// enum values from the DB,
// and the DB shouldn't let us give it
// invalid enum values
// So just casting should be okay?
// I still think having functions to
// make intent clear is good though

func dbPwHashTypeToApp(dbPHT queries.PwHashTypeT) PwHashType {
	return PwHashType(dbPHT)
}
func appPwHashTypeToDB(pht PwHashType) queries.PwHashTypeT {
	return queries.PwHashTypeT(pht)
}

type User struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	Username   string
	PwHashType PwHashType
	PwHash     string
}

func dbUserToApp(in queries.User) *User {
	return &User{
		ID:         in.ID,
		CreatedAt:  in.CreatedAt.Time,
		Username:   in.Username,
		PwHashType: dbPwHashTypeToApp(in.PwHashType),
		PwHash:     in.PwHash,
	}
}

// TODO: temporary, its just easiest to map from DB rn
type SessionToken struct {
	ID        uuid.UUID
	CreatedAt time.Time
	ForUser   uuid.UUID
	Token     string
}

func dbSessionTokenToApp(in queries.SessionToken) *SessionToken {
	return &SessionToken{
		ID:        in.ID,
		CreatedAt: in.CreatedAt.Time,
		ForUser:   in.ForUser,
		Token:     in.Token,
	}
}
