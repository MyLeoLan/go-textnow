package stubs

import (
	"math/rand"
	"strconv"
	"time"

	uuid "github.com/satori/go.uuid"
)

// GetPhoneNumber ...
func GetPhoneNumber() string {
	rand.Seed(time.Now().UnixNano())
	phoneNumber := 0
	digits := 10
	for digits > 0 {
		phoneNumber *= 10
		phoneNumber += rand.Intn(10)
		digits--
	}

	return "+1" + strconv.Itoa(phoneNumber)
}

// GetPhoneNumberWithAreaCode ...
func GetPhoneNumberWithAreaCode(areaCode int) string {
	phoneNumber := GetPhoneNumber()
	return phoneNumber[:2] + strconv.Itoa(areaCode) + phoneNumber[5:]
}

// GetUserID ...
func GetUserID() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(1000) + 1
}

// GetRefID ...
func GetRefID() string {
	return uuid.NewV4().String()
}

// GetIdempotencyKey ..
func GetIdempotencyKey() string {
	return uuid.NewV4().String()
}
