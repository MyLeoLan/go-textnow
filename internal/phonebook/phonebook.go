package phonebook

import (
	context "context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/OmarElGabry/go-textnow/internal/pkg/logger"
	"github.com/OmarElGabry/go-textnow/internal/pkg/mysql"
	"github.com/OmarElGabry/go-textnow/internal/pkg/redis"

	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	db    *mysql.DB
	cache *redis.Cache
	// mu    sync.Mutex
}

// NewPhoneBookServiceServer creates and returns a new PhoneBook service server
func NewPhoneBookServiceServer(db *mysql.DB, cache *redis.Cache) PhoneBookServiceServer {
	return &server{db: db, cache: cache}
}

// FindOne method finds if the given phone number exists or not
func (s *server) FindOne(ctx context.Context, req *FindOneRequest) (*FindOneResponse, error) {
	phoneNumber := req.GetPhoneNumber()

	_, err := s.cache.Get(phoneNumber).Result()
	if err == s.cache.ErrNotExists {
		row := s.db.QueryRow("SELECT user_id FROM phonebook WHERE phone_number=?", phoneNumber)
		var userID int
		err := row.Scan(&userID)

		switch {
		case err == sql.ErrNoRows:
			return &FindOneResponse{Exists: false}, nil
		case err != nil:
			return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
		}

		// keys will be evicted according to "allkeys-lru" policy
		// redis checks the memory usage, and if it is greater than the maxmemory limit,
		// it evicts keys according to that policy.
		_, err = s.cache.Set(phoneNumber, true, 0).Result()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to set key in Redis error: %v", err))
		}

	} else if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	return &FindOneResponse{Exists: true}, nil
}

// Reserve method reservers 5 (unassigned) phone numbers and allow the user to choose one of them.
func (s *server) Reserve(ctx context.Context, req *ReserveRequest) (*ReserveResponse, error) {
	areaCode := req.GetAreaCode()
	refID := uuid.NewV4().String()
	areaCodeKey := "areacode-" + strconv.Itoa(int(areaCode))

	// 1) Get 5 phone numbers by areaCode
	// Redis is actually single-threaded, and so only one command at a time is executed.
	// s.mu.Lock()
	phoneNumbers, err := s.cache.SPopN(areaCodeKey, 5).Result()
	// s.mu.Unlock()

	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Failed to reserve phone numbers: %v", err))
	}

	if len(phoneNumbers) < 5 {
		// re-insert the phone numbers back, no longer going to use them
		s.cache.SAdd(areaCodeKey, phoneNumbers)

		logger.Warn(fmt.Sprintf("Cache is running out of available phone numbers for area code %d", areaCode))
		return nil, status.Errorf(codes.FailedPrecondition, "Not enough available phone numbers!")
	}

	// 2) Add them to refID set to be fetched later in Assign()
	// 	To get the areaCodeKey in Assign()
	//	A small trick is to add "areaCodeKey" at the end
	s.cache.SAdd("refid-"+refID, append(phoneNumbers, areaCodeKey))

	return &ReserveResponse{PhoneNumbers: phoneNumbers, RefId: refID}, nil
}

// Assign method assigns the choosen number to the user
//
// It is called immediately after Reserve method to carry on the phone number assignment.
// NOTE: We assume that the "userID" already exists in the database.
func (s *server) Assign(ctx context.Context, req *AssignRequest) (*AssignResponse, error) {
	phoneNumber := req.GetPhoneNumber()
	userID := req.GetUserId()
	refIDKey := "refid-" + req.GetRefId()

	// 1) Check if the selected phone number & refID exists
	// 	Also keep the un-selected (skipped) numbers aside.
	// 	Remember that for every key "refid-refID", the last value is the areaCodeKey
	phoneNumbersAndAreaCode, err := s.cache.SMembers(refIDKey).Result()
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	found := false
	skippedNumbers := []string{}
	var areaCodeKey string
	for _, pNumberOrAreaCode := range phoneNumbersAndAreaCode {
		if pNumberOrAreaCode == phoneNumber {
			found = true
		} else if strings.HasPrefix(pNumberOrAreaCode, "areacode-") {
			areaCodeKey = pNumberOrAreaCode
		} else {
			skippedNumbers = append(skippedNumbers, pNumberOrAreaCode)
		}
	}

	if !found {
		return nil, status.Error(codes.InvalidArgument, "Phone number and/or reference id is wrong")
	}

	// 2) We no longer going to use that refIDKey key
	_, err = s.cache.Del(refIDKey).Result()
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	// 3) Add the skipped numbers back to the areaCodeKey and so available for selection
	// Remember that phone numbers that are already exist in the Set are ignored.
	_, err = s.cache.SAdd(areaCodeKey, skippedNumbers).Result()
	if err != nil {
		logger.Error(
			fmt.Sprintf("Failed to re-insert skipped numbers %v to %s after %s has been deleted",
				skippedNumbers, areaCodeKey, phoneNumber))
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Internal error: %v", err))
	}

	// 4) Assign the selected number to the user
	// We use database for "phonebook": A table of users and their info.
	_, err = s.db.Exec("UPDATE phonebook SET phone_number=? WHERE user_id=?", phoneNumber, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			fmt.Sprintf("Failed to assign the phone number: %v", err))
	}

	// 5) Update the cache so that subsequent request result in cache hit
	// We could, however, store it in the cache, and have an async queue to update the database.
	_, err = s.cache.Set(phoneNumber, true, 0).Result()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to set key in Redis error: %v", err))
	}

	return &AssignResponse{Assigned: true}, nil
}
