package auth

// Shared Mongo filter builders for `auth_tokens`. The user-scope
// pattern below appears wherever an admin handler asks "all rows
// belonging to this user" — it has to OR `user_id` (modern rows)
// with `email` (legacy rows minted before user_id was stamped) so
// historical data isn't invisible. Centralised here so a future
// schema change (drop the legacy email branch, add a soft-delete
// gate, etc.) lands in one place.

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/models"
)

// userScopeFilter returns a `auth_tokens` filter that matches
// every row belonging to the given user, narrowed by `scope`.
//
// Shape of the result:
//
//	{
//	   <scope keys>...,
//	   "$or": [{"user_id": u.ID}, {"email": u.Email}],
//	}
//
// `scope` carries the discriminator the caller wants (e.g.
// `bson.M{"test": true}` for test-link queries, or
// `bson.M{"type": models.TokenTypeJWT}` for JWT-session queries).
// Callers MUST always pass a scope — without one the filter would
// match every row in the collection regardless of type, which is
// almost never what an admin endpoint wants.
//
// Returns a fresh map so the caller's `scope` argument isn't
// mutated; safe to reuse the same `scope` literal across calls.
func userScopeFilter(u *models.User, scope bson.M) bson.M {
	f := make(bson.M, len(scope)+1)
	for k, v := range scope {
		f[k] = v
	}
	f["$or"] = []bson.M{
		{"user_id": u.ID},
		{"email": u.Email},
	}
	return f
}
