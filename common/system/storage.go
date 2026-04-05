package system

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
)

var (
	storageOnce sync.Once
	storage     *Storage
)

// Storage wraps a MongoDB database and exposes generic CRUD operations.
// All scout platforms use this instead of talking to mongo directly.
type Storage struct {
	db *mongo.Database
}

type StorageIndexSpec struct {
	Collection string
	Field      string
	Unique     bool
}

func NewIndexSpec(collection, field string, unique bool) StorageIndexSpec {
	return StorageIndexSpec{Collection: collection, Field: field, Unique: unique}
}

// GetStorage returns the process-wide Storage handle.
// InitStorage must be called before GetStorage.
func GetStorage() *Storage {
	return storage
}

// InitStorage connects to MongoDB and stores the singleton storage handle.
// Reads MONGODB_URI and MONGODB_DATABASE from the environment.
// Must be called once from main after system.Init().
func InitStorage(ctx context.Context) error {
	var initErr error
	storageOnce.Do(func() {
		values, err := helpers.ReadEnvs("MONGODB_URI", "MONGODB_DATABASE")
		if err != nil {
			initErr = err

			return
		}

		uri, dbName := values[0], values[1]
		client, err := mongo.Connect(options.Client().ApplyURI(uri))
		if err != nil {
			initErr = err
			return
		}

		if err := client.Ping(ctx, nil); err != nil {
			initErr = err
			return
		}
		s := &Storage{db: client.Database(dbName)}
		spec := NewIndexSpec(constants.MongoProjectsCollection, "id", true)
		if err := s.EnsureIndex(ctx, spec); err != nil {
			initErr = err
			return
		}
		storage = s
	})
	return initErr
}

func (s *Storage) GetById(ctx context.Context, collection string, id string, result any) error {
	filter := bson.M{"id": id}
	return s.Get(ctx, collection, filter, result)
}

// Get finds a single document matching filter and decodes it into result.
func (s *Storage) Get(ctx context.Context, collection string, filter bson.M, result any) error {
	return s.db.Collection(collection).FindOne(ctx, filter).Decode(result)
}

func (s *Storage) SetById(ctx context.Context, collection string, id string, doc any) error {
	filter := bson.M{"id": id}
	return s.Set(ctx, collection, filter, doc)
}

// Set partially updates a document ($set): only the provided fields are written,
// existing fields not present in doc are left untouched. Inserts if not found.
func (s *Storage) Set(ctx context.Context, collection string, filter bson.M, doc any) error {
	_, err := s.db.Collection(collection).UpdateOne(
		ctx, filter, bson.M{"$set": doc}, options.UpdateOne().SetUpsert(true),
	)
	return err
}

// Replace fully replaces the document matching filter with doc, removing any
// fields that are no longer present. Inserts if not found.
// Filters by internal id when available (stable across updates); falls back to
// platform_id + platform as the natural key on first insert.
func (s *Storage) Replace(ctx context.Context, collection string, doc *models.PersistedProject) error {
	var filter bson.M
	if doc.ID != "" {
		filter = bson.M{"id": doc.ID}
	} else {
		filter = bson.M{"platform_id": doc.GetPlatformId(), "platform": doc.GetPlatform()}
	}
	_, err := s.db.Collection(collection).ReplaceOne(
		ctx, filter, doc, options.Replace().SetUpsert(true),
	)
	return err
}

// GetMany finds all documents matching a plain map filter and decodes them into
// results (must be a pointer to a slice, e.g. *[]MyStruct).
// The map is converted to a bson filter internally — callers do not need to import bson.
func (s *Storage) GetManyByField(ctx context.Context, collection string, field string, targets []string, results any) error {
	filter := bson.M{}
	filter[field] = bson.M{"$in": targets}
	return s.GetMany(ctx, collection, filter, results)
}

// GetManyRaw is like GetMany but accepts an explicit bson.M filter, useful when
// the filter contains bson operators such as $in, $gt, etc.
func (s *Storage) GetMany(ctx context.Context, collection string, filter bson.M, results any) error {
	cursor, err := s.db.Collection(collection).Find(ctx, filter)
	if err != nil {
		return err
	}
	return cursor.All(ctx, results)
}

// EnsureIndex creates an index on field in collection if it does not already exist.
// Pass unique=true to enforce uniqueness.
func (s *Storage) EnsureIndex(ctx context.Context, spec StorageIndexSpec) error {
	model := mongo.IndexModel{
		Keys:    bson.D{{Key: spec.Field, Value: 1}},
		Options: options.Index().SetUnique(spec.Unique),
	}
	_, err := s.db.Collection(spec.Collection).Indexes().CreateOne(ctx, model)
	return err
}

// UpsertDoc pairs a filter with the document to upsert in a SetMany call.
type UpsertDoc struct {
	Filter bson.M
	Doc    any
}

// SetMany upserts multiple documents in a single bulk write operation.
func (s *Storage) SetMany(ctx context.Context, collection string, docs []UpsertDoc) error {
	models := make([]mongo.WriteModel, len(docs))
	for i, d := range docs {
		m := mongo.NewUpdateOneModel()
		m.SetFilter(d.Filter)
		m.SetUpdate(bson.M{"$set": d.Doc})
		m.SetUpsert(true)
		models[i] = m
	}
	_, err := s.db.Collection(collection).BulkWrite(ctx, models)
	return err
}

// Insert adds a new document to collection without upsert semantics.
func (s *Storage) Insert(ctx context.Context, collection string, doc any) error {
	_, err := s.db.Collection(collection).InsertOne(ctx, doc)
	return err
}
