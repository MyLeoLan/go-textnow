package sms

import (
	context "context"
	fmt "fmt"
	"io"
	"time"

	"github.com/OmarElGabry/go-callme/internal/phonebook"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type server struct {
	db *mongo.Collection
	pB phonebook.PhoneBookServiceClient
}

// NewSMSServiceServer creates and returns a new SMS service server
func NewSMSServiceServer(db *mongo.Collection, pB phonebook.PhoneBookServiceClient) SMSServiceServer {
	return &server{db, pB}
}

// SendOne method sends a single sms
func (s *server) SendOne(ctx context.Context, req *SendOneRequest) (*SendOneResponse, error) {
	smsReq := req.GetSms()
	idempotencyKey := smsReq.GetIdempotencyKey()
	fromPhoneNumber := smsReq.GetFromPhoneNumber()
	toPhoneNumber := smsReq.GetToPhoneNumber()
	content := smsReq.GetContent()

	// 1) Check idempotency
	filter := bson.M{"idempotencyKey": idempotencyKey}
	idempotent, err := s.isIdempotent(ctx, filter)

	if err != nil {
		return nil, status.Error(codes.Internal, "Internal error "+err.Error())
	}

	if idempotent == false {
		// If we returned: nil, status.Error(codes.OK, "...")
		// status.Error() return nil if OK. And response must not be "nil"!.
		return &SendOneResponse{Sent: true, Message: "Message has been sent already"}, nil
	}

	// make sure to delete the created document (@isIdempotent()) upon failure
	defer func() {
		// it assumes that when SendOne() returns on error,
		// it is ONLY when err is != nil. if SendOne() returned on failure
		// and err was nil (i.e. invalid input), the document won't be deleted!.
		if err != nil {
			s.db.DeleteOne(ctx, filter)
		}
	}()

	// 2) Check if phone numbers actually exist in the database
	err = s.findPhoneNumber(ctx, fromPhoneNumber)
	if err != nil {
		return nil, err
	}

	err = s.findPhoneNumber(ctx, toPhoneNumber)
	if err != nil {
		return nil, err
	}

	// 3) Send the sms: Add sms to databsae by updating the the created document (@isIdempotent())
	// We simulate "sending sms" by inserting it to the database.
	_, err = s.db.UpdateOne(ctx, filter, bson.M{"$set": bson.M{
		"from":    fromPhoneNumber,
		"to":      toPhoneNumber,
		"content": content,
	}})

	if err != nil {
		return nil, status.Error(codes.Internal, "Internal error "+err.Error())
	}

	return &SendOneResponse{Sent: true}, nil
}

// SendMany method sends many SMSs in one request.
func (s *server) SendMany(stream SMSService_SendManyServer) error {

	errors := []string{}
	for {
		// For each sms, ...
		req, err := stream.Recv() // blocks!
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Error(codes.Internal, "Internal error "+err.Error())
		}

		// Send each using SendOne method. If error returned, append it to errors array
		_, err = s.SendOne(stream.Context(), &SendOneRequest{Sms: req.GetSms()})

		if err != nil {
			errors = append(errors,
				fmt.Sprintf("Couldn't send sms %v \n error %v", req.GetSms(), err))
		}

		// sleep to avoid overwhelming SendOne method.
		time.Sleep(100 * time.Millisecond)
	}

	return stream.SendAndClose(&SendManyResponse{Errors: errors})
}

// isIdempotent is a helper function to check if the SMS is idempotent (has been sent before) or not.
// Ideally, this should be in a middelware:
//	- A pre-middleware to check if key is idempotent and create it if not.
//	- A post-middelware to store the result (response).
//
// The client is expected to pass that idempotent key in the request. An example is to use UUID V4.
func (s *server) isIdempotent(ctx context.Context, idempotencyFilter bson.M) (bool, error) {
	data := bson.M{"$set": idempotencyFilter}
	upsert := true // create it if not exists

	// UpdateOne is used instead of InsertOne because it is easier
	// to check if sms with the same idempotency key already exists or not.

	// var mu sync.Mutex
	// mu.Lock()
	res, err := s.db.UpdateOne(ctx, idempotencyFilter, data, &options.UpdateOptions{Upsert: &upsert})
	// mu.Unlock()

	if err != nil {
		return false, err
	}

	// already exists
	if res.UpsertedCount == 0 {
		return false, nil
	}

	return true, nil
}

// findPhoneNumber is a helper function to find
// if a given phone number exists by calling FindOne of PhoneBook service
func (s *server) findPhoneNumber(ctx context.Context, phoneNumber string) error {
	res, err := s.pB.FindOne(ctx, &phonebook.FindOneRequest{PhoneNumber: phoneNumber})
	if err != nil {
		return status.Error(codes.Internal, "Internal error "+err.Error())
	}

	if res.GetExists() == false {
		return status.Error(codes.NotFound, "Phone number doesn't exist")
	}

	return nil
}
