package tests

import (
	"context"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"google.golang.org/grpc/codes"

	"github.com/OmarElGabry/go-textnow/internal/sms"
	"github.com/OmarElGabry/go-textnow/tests/stubs"
)

func TestSMS(t *testing.T) {
	uri := "http://gateway-service:8080/sms/"

	// clear database
	// sms doesn't access MySQL database but relies on having existing phonenumber records
	DropMongoDB()
	TruncateMySQL()

	t.Run("TestSendOne", func(t *testing.T) {
		// 1) test when phone numbers don't exist
		idempotencyKey := stubs.GetIdempotencyKey()
		fromPhoneNumber := stubs.GetPhoneNumber()
		toPhoneNumber := stubs.GetPhoneNumber()

		postData, err := CreateRequest(&sms.SendOneRequest{
			Sms: &sms.SMS{
				IdempotencyKey:  idempotencyKey,
				FromPhoneNumber: fromPhoneNumber,
				ToPhoneNumber:   toPhoneNumber,
				Content:         "content of the sms",
			},
		})

		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err := http.Post(uri+"send/one", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		var errorMsg ErrorBody
		err = ReadError(res.Body, &errorMsg)
		if err != nil {
			t.Errorf("failed to read error body %v; want success", err)
			return
		}

		if got, want := errorMsg.Code, int(codes.NotFound); got != want {
			t.Errorf("msg.Code = %d; want %d", got, want)
			return
		}

		// check if db is empty
		// it is a good practice to check the database even if we got the expected response
		filter := bson.M{}
		cunt, err := dbMongo.CountDocuments(context.TODO(), filter)
		if err != nil {
			t.Errorf("failed to get number of documents in db %v; want success", err)
			return
		}

		if got, want := cunt, int64(0); got != want {
			t.Errorf("number of documents in db = %d; want %d", got, want)
			return
		}

		// 2) test with existing, valid phone numbers
		_, err = dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, ?)",
			stubs.GetUserID(), fromPhoneNumber)
		if err != nil {
			t.Errorf("couldn't insert 'from' phone number: %v", err)
			return
		}

		_, err = dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, ?)",
			stubs.GetUserID(), toPhoneNumber)
		if err != nil {
			t.Errorf("couldn't insert 'to' phone number: %v", err)
			return
		}

		postData, err = CreateRequest(&sms.SendOneRequest{
			Sms: &sms.SMS{
				IdempotencyKey:  idempotencyKey,
				FromPhoneNumber: fromPhoneNumber,
				ToPhoneNumber:   toPhoneNumber,
				Content:         "content of the sms",
			},
		})

		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err = http.Post(uri+"send/one", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		var resData sms.SendOneResponse
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
		}

		if got, want := resData.Sent, true; got != want {
			t.Errorf("Sent = %t; want %t", got, want)
		}

		// check database
		cunt, err = dbMongo.CountDocuments(context.TODO(), filter)
		if err != nil {
			t.Errorf("failed to get number of documents in db %v; want success", err)
			return
		}

		if got, want := cunt, int64(1); got != want {
			t.Errorf("number of documents in db = %d; want %d", got, want)
			return
		}

		// 3) test with existing idempotency key
		// postData must be created again. It loses its values once its used.
		postData, err = CreateRequest(&sms.SendOneRequest{
			Sms: &sms.SMS{
				IdempotencyKey:  idempotencyKey,
				FromPhoneNumber: fromPhoneNumber,
				ToPhoneNumber:   toPhoneNumber,
				Content:         "content of the sms",
			},
		})

		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}
		res, err = http.Post(uri+"send/one", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		resData = sms.SendOneResponse{}
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
			return
		}

		if got, want := resData.Sent, true; got != want {
			t.Errorf("Sent = %t; want %t", got, want)
			return
		}

		if got, want := len(resData.Message) > 0, true; got != want {
			t.Errorf("Message string is missing")
			return
		}
	})

	t.Run("TestSendMany", func(t *testing.T) {
		// we need to wipe out all the data
		// if we're going to check the number of database records
		// Or, make sure to check number of records that match a condition i.e. idempotency key
		DropMongoDB()

		// create sms
		fromPhoneNumber := stubs.GetPhoneNumber()
		toPhoneNumber := stubs.GetPhoneNumber()

		// insert phone numbers in the database
		_, err := dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, ?)",
			stubs.GetUserID(), fromPhoneNumber)
		if err != nil {
			t.Errorf("couldn't insert 'from' phone number: %v", err)
			return
		}

		_, err = dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, ?)",
			stubs.GetUserID(), toPhoneNumber)
		if err != nil {
			t.Errorf("couldn't insert 'to' phone number: %v", err)
			return
		}

		// for simplicity, we'll just send two sms with same phone numbers
		// but different idempotency key and content
		postData, err := CreateRequest(&sms.SendOneRequest{
			Sms: &sms.SMS{
				IdempotencyKey:  stubs.GetIdempotencyKey(),
				FromPhoneNumber: fromPhoneNumber,
				ToPhoneNumber:   toPhoneNumber,
				Content:         "content of the sms 1",
			},
		}, &sms.SendOneRequest{
			Sms: &sms.SMS{
				IdempotencyKey:  stubs.GetIdempotencyKey(),
				FromPhoneNumber: fromPhoneNumber,
				ToPhoneNumber:   toPhoneNumber,
				Content:         "content of the sms 2",
			},
		})

		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err := http.Post(uri+"send/many", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		var resData sms.SendManyResponse
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
		}

		if got, want := len(resData.Errors), 0; got != want {
			t.Errorf("number of errors = %d; want %d", got, want)
		}

		// check database
		filter := bson.M{}
		cunt, err := dbMongo.CountDocuments(context.TODO(), filter)
		if err != nil {
			t.Errorf("failed to get number of documents in db %v; want success", err)
			return
		}

		if got, want := cunt, int64(2); got != want {
			t.Errorf("number of documents in db = %d; want %d", got, want)
			return
		}
	})
}
