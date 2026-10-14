package fleet

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

type mongoDocument struct {
	Vehicle `bson:",inline"`
	Outbox  []Event `bson:"outbox"`
}
type MongoRepository struct {
	Client     *mongo.Client
	collection *mongo.Collection
}

func OpenMongo(ctx context.Context, uri, database string) (*MongoRepository, error) {
	c, err := mongo.Connect(options.Client().ApplyURI(uri).SetMaxPoolSize(48).SetServerSelectionTimeout(2 * time.Second).SetConnectTimeout(2 * time.Second))
	if err != nil {
		return nil, errors.New("MongoDB configuration invalid")
	}
	repo := &MongoRepository{Client: c, collection: c.Database(database).Collection("vehicles")}
	if err = repo.Ready(ctx); err != nil {
		_ = c.Disconnect(context.Background())
		return nil, errors.New("MongoDB unavailable")
	}
	_, err = repo.collection.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "active", Value: 1}, {Key: "dailyRateMinor", Value: 1}, {Key: "_id", Value: 1}}})
	if err != nil {
		_ = c.Disconnect(context.Background())
		return nil, errors.New("MongoDB index creation failed")
	}
	return repo, nil
}
func (r *MongoRepository) Ready(ctx context.Context) error { return r.Client.Ping(ctx, nil) }
func (r *MongoRepository) Close(ctx context.Context) error { return r.Client.Disconnect(ctx) }
func (r *MongoRepository) Get(ctx context.Context, id string) (Vehicle, error) {
	var d mongoDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"outbox": 0})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Vehicle{}, ErrNotFound
	}
	if err != nil {
		return Vehicle{}, errors.New("MongoDB read failed")
	}
	return d.Vehicle, nil
}
func (r *MongoRepository) List(ctx context.Context, q Query) (Page, error) {
	filter := bson.M{}
	if !q.IncludeInactive {
		filter["active"] = true
	}
	if q.Brand != "" {
		filter["brand"] = q.Brand
	}
	if q.Category != "" {
		filter["category"] = q.Category
	}
	if q.Transmission != "" {
		filter["transmission"] = q.Transmission
	}
	if q.Seats > 0 {
		filter["seats"] = q.Seats
	}
	sort := bson.D{{Key: "dailyRateMinor", Value: 1}, {Key: "_id", Value: 1}}
	if q.Sort == "price_desc" {
		sort[0].Value = -1
	}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return Page{}, errors.New("MongoDB count failed")
	}
	cursor, err := r.collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"outbox": 0}).SetSort(sort).SetSkip(int64((q.Page-1)*q.PageSize)).SetLimit(int64(q.PageSize)))
	if err != nil {
		return Page{}, errors.New("MongoDB list failed")
	}
	defer cursor.Close(ctx)
	var docs []mongoDocument
	if err = cursor.All(ctx, &docs); err != nil {
		return Page{}, errors.New("MongoDB decode failed")
	}
	out := Page{Items: make([]Vehicle, 0, len(docs)), Total: total, Page: q.Page, PageSize: q.PageSize}
	for _, d := range docs {
		out.Items = append(out.Items, d.Vehicle)
	}
	return out, nil
}
func (r *MongoRepository) Create(ctx context.Context, v Vehicle, e Event) (Vehicle, error) {
	_, err := r.collection.InsertOne(ctx, mongoDocument{Vehicle: v, Outbox: []Event{e}})
	if mongo.IsDuplicateKeyError(err) {
		return Vehicle{}, ErrConflict
	}
	if err != nil {
		return Vehicle{}, errors.New("MongoDB create failed")
	}
	return v, nil
}
func (r *MongoRepository) Update(ctx context.Context, v Vehicle, expected int64, e Event) (Vehicle, error) {
	set := bson.M{"brand": v.Brand, "model": v.Model, "category": v.Category, "transmission": v.Transmission, "seats": v.Seats, "dailyRateMinor": v.DailyRateMinor, "currency": v.Currency, "active": v.Active, "version": v.Version, "updatedAt": v.UpdatedAt}
	filter := bson.M{"_id": v.VehicleID, "version": expected, fmt.Sprintf("outbox.%d", MaxPendingEvents-1): bson.M{"$exists": false}}
	var d mongoDocument
	err := r.collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": set, "$push": bson.M{"outbox": e}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		var existing mongoDocument
		err = r.collection.FindOne(ctx, bson.M{"_id": v.VehicleID}).Decode(&existing)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Vehicle{}, ErrNotFound
		}
		if err != nil {
			return Vehicle{}, errors.New("MongoDB update check failed")
		}
		if existing.Version != expected {
			return Vehicle{}, ErrConflict
		}
		if len(existing.Outbox) >= MaxPendingEvents {
			return Vehicle{}, ErrOutboxFull
		}
		return Vehicle{}, ErrConflict
	}
	if err != nil {
		return Vehicle{}, errors.New("MongoDB update failed")
	}
	return d.Vehicle, nil
}
func (r *MongoRepository) Pending(ctx context.Context, limit int) ([]Event, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"outbox.0": bson.M{"$exists": true}}}},
		bson.D{{Key: "$unwind", Value: "$outbox"}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "outbox.occurredAt", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
		bson.D{{Key: "$replaceRoot", Value: bson.M{"newRoot": "$outbox"}}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, errors.New("MongoDB outbox read failed")
	}
	defer cursor.Close(ctx)
	events := []Event{}
	if err = cursor.All(ctx, &events); err != nil {
		return nil, errors.New("MongoDB outbox decode failed")
	}
	return events, nil
}
func (r *MongoRepository) MarkPublished(ctx context.Context, e Event) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": e.VehicleID}, bson.M{"$pull": bson.M{"outbox": bson.M{"eventId": e.EventID}}})
	if err != nil {
		return errors.New("MongoDB outbox acknowledgement failed")
	}
	return nil
}
func (r *MongoRepository) EnqueueRebuild(ctx context.Context, id string, e Event) error {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id, fmt.Sprintf("outbox.%d", MaxPendingEvents-1): bson.M{"$exists": false}}, bson.M{"$push": bson.M{"outbox": e}})
	if err != nil {
		return errors.New("MongoDB rebuild enqueue failed")
	}
	if result.MatchedCount == 0 {
		return ErrOutboxFull
	}
	return nil
}

// The immutable _id ordering avoids offset shifts when prices change during a rebuild.
// Concurrent creations/updates also retain their own outbox event.
func (r *MongoRepository) Scan(ctx context.Context, visit func(Vehicle) error) error {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"outbox": 0}).SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return errors.New("MongoDB rebuild scan failed")
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc mongoDocument
		if err := cursor.Decode(&doc); err != nil {
			return errors.New("MongoDB rebuild decode failed")
		}
		if err := visit(doc.Vehicle); err != nil {
			return err
		}
	}
	if err := cursor.Err(); err != nil {
		return errors.New("MongoDB rebuild cursor failed")
	}
	return nil
}
func SeedDemo(ctx context.Context, r Repository) error {
	active := true
	fixtures := []Input{
		{VehicleID: "auto-001", Brand: "Marca Demo", Model: "Sedán Uno", Category: "sedan", Transmission: "manual", Seats: 5, DailyRateMinor: 5000000, Currency: "ARS", Active: &active},
		{VehicleID: "auto-002", Brand: "Marca Demo", Model: "SUV Dos", Category: "suv", Transmission: "automatic", Seats: 5, DailyRateMinor: 6500000, Currency: "ARS", Active: &active},
	}
	s := NewService(r, nil, nil)
	for _, in := range fixtures {
		if _, err := s.Create(ctx, in); err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return nil
}
