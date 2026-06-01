package mongodb

import (
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
)

func buildRegexQuery(value string) bson.M {
	return bson.M{"$regex": regexp.QuoteMeta(value)}
}
