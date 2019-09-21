package phonebook

import (
	context "context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/OmarElGabry/go-callme/internal/pkg/logger"

	"github.com/OmarElGabry/go-callme/internal/pkg/mysql"

	uuid "github.com/satori/go.uuid"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	db *mysql.DB
}

// NewPhoneBookServiceServer creates and returns a new PhoneBook service server
func NewPhoneBookServiceServer(db *mysql.DB) PhoneBookServiceServer {
	return &server{db}
}

// FindOne method finds if the given phone number exists or not
func (s *server) FindOne(ctx context.Context, req *FindOneRequest) (*FindOneResponse, error) {
	phoneNumber := req.GetPhoneNumber()

	row := s.db.QueryRow("SELECT user_id FROM phonebook WHERE phone_number=?", phoneNumber)
	var userID int
	err := row.Scan(&userID)

	switch {
	case err == sql.ErrNoRows:
		return &FindOneResponse{Exists: false}, nil
	case err != nil:
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	return &FindOneResponse{Exists: true}, nil
}

// Reserve method reservers 5 (unassigned) phone numbers and allow the user to choose one of them.
func (s *server) Reserve(ctx context.Context, req *ReserveRequest) (*ReserveResponse, error) {
	// 1) Update 5 phone numbers with the given area code and set their refID and status
	areaCode := req.GetAreaCode()
	refID := uuid.NewV4().String()

	ctx, span := trace.StartSpan(ctx, "Reserve 5 numbers UPDATE statement")

	// For one UPDATE statement, the database engine ensures a row is updated by one thread at a time.
	// var mu sync.Mutex
	// mu.Lock()
	res, err := s.db.Exec("UPDATE un_assigned_numbers SET ref_id=?, timestamp=?, status='INUSE' WHERE area_code=? AND status='AVAILABLE' LIMIT 5;",
		refID, time.Now().Unix(), areaCode)
	// mu.Unlock()
	span.End()

	// check for errors ...
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to reserve phone numbers: %v", err))
	}

	updatedRows, err := res.RowsAffected()
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to reserve phone numbers: %v", err))
	}

	// make sure we have 5 numbers that have been updated
	// keep in mind that rows will be updated and assigned ref_id anyway.
	if updatedRows < 5 {
		logger.Warn("Database is running out of available phone numbers less than 5!")
		return nil, status.Errorf(codes.FailedPrecondition, "Not enough available phone numbers!")
	}

	// 2) Get the reserved phone numbers
	// 	this query doesn't need to be locked since
	// 	the phone numbers have been reserved already by changing status to INUSE
	rows, err := s.db.Query("SELECT phone_number FROM un_assigned_numbers WHERE ref_id=?", refID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to reserve phone numbers: %v", err))
	}

	// will close automatically when rows.Next() loop terminates
	// unless we exited the loop early
	defer rows.Close()

	// 3) Fetch the numbers
	phoneNumbers := []string{}
	for rows.Next() {
		var phoneNum string
		err := rows.Scan(&phoneNum)
		if err != nil {
			return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to fetch the phone number from the result set: %v", err))
		}

		phoneNumbers = append(phoneNumbers, phoneNum)
	}

	// rows.Next() returns false if there is no next result row or an error
	// happened while preparing it. Err should be consulted to distinguish between the two cases.
	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to reserve phone numbers: %v", err))
	}

	return &ReserveResponse{PhoneNumbers: phoneNumbers, RefId: refID}, nil
}

// Assign method assigns the choosen number to the user
//
// It is called immediately after Reserve method to carry on the phone number assignment.
// NOTE: We assume that the "userID" already exists in the database.
func (s *server) Assign(ctx context.Context, req *AssignRequest) (*AssignResponse, error) {
	refID := req.GetRefId()
	phoneNumber := req.GetPhoneNumber()
	userID := req.GetUserId()

	// 1) Check if the given choosen phone number & refID exists in the database or not
	row := s.db.QueryRow("SELECT phone_number FROM un_assigned_numbers WHERE ref_id=? AND phone_number=?", refID, phoneNumber)

	var dbPhoneNumber string
	err := row.Scan(&dbPhoneNumber)
	switch {
	case err == sql.ErrNoRows:
		return nil, status.Error(codes.InvalidArgument,
			"Phone number and/or reference id is wrong or phone number has been taken!")
	case err != nil:
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	// 2) Update all un-choosen numbers back to AVAILABLE and clear the ref_id column
	// 	from indexes stand point, empty value is faster than NULL
	_, err = s.db.Exec("UPDATE un_assigned_numbers SET status='AVAILABLE', ref_id='', timestamp=NULL WHERE ref_id=? AND phone_number!=?",
		refID, phoneNumber)

	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to assign the phone number: %v", err))
	}

	// ----- Execute the following in a transaction -----
	// Must ensure that all of them successfully committed or rollback on failure.
	// 	Instead of using transactions, another way of doing it is to,
	//		rather than deleting, just flag it as "ASSIGNED" so other users won't take it.
	// 	Then a background process will re-try to do it later if failed and delete it when successful.

	ctx, span := trace.StartSpan(ctx, "Transaction: assign the choosen number")
	defer span.End()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	// Commit and rollback on failure
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
	}()

	// The below SQL statements can run concurrently; they don't need to be in sequence
	var wg sync.WaitGroup
	errChan := make(chan error)
	wg.Add(2)

	// 3) Delete the choosen phone number.
	// 	Now, there is supposed to be only one phone number with the refID
	go func() {
		_, err = tx.Exec("DELETE FROM un_assigned_numbers WHERE ref_id=?", refID)
		errChan <- err
		wg.Done()
	}()

	// 4) Assign it to the user
	// 	Again, we assume that the userID already exists.
	go func() {
		_, err = tx.Exec("UPDATE phonebook SET phone_number=? WHERE user_id=?", phoneNumber, userID)
		errChan <- err
		wg.Done()
	}()

	// collect the errors if any
	go func() {
		wg.Wait()
		close(errChan)
	}()

	for ec := range errChan {
		if ec != nil && err == nil { // take the first error
			// we can return or break the loop!
			// this loop needs to finish so that there won't be any leaky gorountines
			// in other words, all goroutines must finish and channel must be closed before returning
			err = ec
		}
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal,
			fmt.Sprintf("Failed to assign the phone number: %v", err))
	}

	err = tx.Commit()
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to assign the phone number: %v", err))
	}

	return &AssignResponse{Assigned: true}, nil
}
