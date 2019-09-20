package tests

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"

	pb "github.com/OmarElGabry/go-callme/internal/phonebook"
	"github.com/OmarElGabry/go-callme/tests/stubs"
)

func TestPhoneBook(t *testing.T) {
	const uri = "http://gateway:8080/phonebook/"

	// clear database once before running tests
	// tests should be independent, and so either clear the database before every test
	// or, use random data. We create stubs we return dummy random data.
	TruncateMySQL()

	// Run the tests as a sub-tests
	t.Run("FindOneWithNonExistingPhoneNumber", func(t *testing.T) {
		phoneNumber := stubs.GetPhoneNumber()
		res, err := http.Get(uri + "find/" + phoneNumber)
		if err != nil {
			t.Errorf("http.Get failed with %v", err)
			return
		}
		defer res.Body.Close()

		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Errorf("resp.StatusCode = %d; want %d", got, want)
			return
		}

		var resData pb.FindOneResponse
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
		}

		if got, want := resData.Exists, false; got != want {
			t.Errorf("exists = %t; want = %t", got, want)
		}
	})

	t.Run("TestFindOneWithExistingPhoneNumber", func(t *testing.T) {
		// create the phone number
		phoneNumber := stubs.GetPhoneNumber()
		userID := stubs.GetUserID()
		_, err := dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, ?)", userID, phoneNumber)
		if err != nil {
			t.Errorf("couldn't insert phone number: %v", err)
			return
		}

		// check if number exists
		res, err := http.Get(uri + "find/" + phoneNumber)
		if err != nil {
			t.Errorf("http.Get failed with %v", err)
			return
		}
		defer res.Body.Close()

		if got, want := res.StatusCode, http.StatusOK; got != want {
			t.Errorf("resp.StatusCode = %d; want %d", got, want)
			return
		}

		var resData pb.FindOneResponse
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
			return
		}

		if got, want := resData.Exists, true; got != want {
			t.Errorf("exists = %t; want = %t", got, want)
			return
		}
	})

	t.Run("TestReserveAndAssign", func(t *testing.T) {
		// ---- reserve
		// create 5 phone numbers to be reserved
		areaCode := 613
		numOfPhoneNumbers := 5

		for i := 0; i < numOfPhoneNumbers; i++ {
			phoneNumber := stubs.GetPhoneNumberWithAreaCode(areaCode)
			_, err := dbMySQL.Exec("INSERT INTO un_assigned_numbers (phone_number, area_code) VALUES (?, ?)",
				phoneNumber, areaCode)
			if err != nil {
				t.Errorf("couldn't insert phone number: %v", err)
				return
			}
		}

		// send a request request to reserve 5 phone numbers
		postData, err := CreateRequest(&pb.ReserveRequest{AreaCode: int32(areaCode)})
		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err := http.Post(uri+"reserve", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		var resData pb.ReserveResponse
		err = ReadRespone(res.Body, &resData)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
		}

		// validate the reserved numbers
		phoneNumbers := resData.PhoneNumbers
		if got, want := len(phoneNumbers), numOfPhoneNumbers; got != want {
			t.Errorf("length of phone numbers = %d; want = %d", got, want)
		}

		refID := resData.RefId
		if got, want := (len(refID) > 0), true; got != want {
			t.Errorf("ref id has length = %t; want len > 0", got)
		}

		// ---- assign
		// first create userID in the database
		userID := int32(stubs.GetUserID())
		_, err = dbMySQL.Exec("INSERT INTO phonebook (user_id, phone_number) VALUES (?, NULL)", userID)
		if err != nil {
			t.Errorf("couldn't insert new user: %v", err)
			return
		}

		// 1) test when refID and phonenumber is wrong
		wrongPhoneNumber := stubs.GetPhoneNumber()
		postData, err = CreateRequest(&pb.AssignRequest{
			PhoneNumber: wrongPhoneNumber,
			RefId:       refID,
			UserId:      userID,
		})
		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err = http.Post(uri+"assign", "application/json", postData)
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

		if got, want := errorMsg.Code, int(codes.InvalidArgument); got != want {
			t.Errorf("msg.Code = %d; want %d", got, want)
			return
		}

		// 2) test when refID is wrong
		wrongRefID := stubs.GetRefID()
		postData, err = CreateRequest(&pb.AssignRequest{
			PhoneNumber: phoneNumbers[0], // choose any number
			RefId:       wrongRefID,
			UserId:      userID,
		})
		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err = http.Post(uri+"assign", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		err = ReadError(res.Body, &errorMsg)
		if err != nil {
			t.Errorf("failed to read error body %v; want success", err)
			return
		}

		if got, want := errorMsg.Code, int(codes.InvalidArgument); got != want {
			t.Errorf("msg.Code = %d; want %d", got, want)
			return
		}

		// 3) test using correct values
		postData, err = CreateRequest(&pb.AssignRequest{
			PhoneNumber: phoneNumbers[0],
			RefId:       refID,
			UserId:      userID,
		})
		if err != nil {
			t.Fatalf("failed to write request body %v; want success", err)
			return
		}

		res, err = http.Post(uri+"assign", "application/json", postData)
		if err != nil {
			t.Errorf("http.Post failed with %v", err)
			return
		}
		defer res.Body.Close()

		var resDataAssign pb.AssignResponse
		err = ReadRespone(res.Body, &resDataAssign)
		if err != nil {
			t.Errorf("reading res.Body failed with %v", err)
		}

		if got, want := resDataAssign.Assigned, true; got != want {
			t.Errorf("Assigned = %t; want %t", got, want)
			return
		}
	})
}
