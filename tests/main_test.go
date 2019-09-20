package tests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"encoding/json"
	"io"
	"io/ioutil"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/OmarElGabry/go-callme/internal/pkg/config"
	"github.com/OmarElGabry/go-callme/internal/pkg/mongodb"
	"github.com/OmarElGabry/go-callme/internal/pkg/mysql"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

var dbMySQL *mysql.DB
var dbMongo *mongo.Collection

// ErrorBody represents the JSON error we get back in the response
type ErrorBody struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TruncateMySQL truncates all tables
func TruncateMySQL() {
	_, err := dbMySQL.Exec("TRUNCATE TABLE phonebook")
	if err != nil {
		log.Fatalf("Failed to truncate table: %v", err)
	}

	_, err = dbMySQL.Exec("TRUNCATE TABLE un_assigned_numbers")
	if err != nil {
		log.Fatalf("Failed to truncate table: %v", err)
	}
}

// DropMongoDB drops all collections
func DropMongoDB() {
	err := dbMongo.Drop(context.TODO())
	if err != nil {
		log.Fatalf("Failed to drop collection: %v", err)
	}
}

// ReadRespone parses the incoming HTTP response to a response struct defined in .proto file
func ReadRespone(source io.ReadCloser, dest proto.Message) error {
	buf, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}

	if err := jsonpb.UnmarshalString(string(buf), dest); err != nil {
		return err
	}

	return nil
}

// ReadError reads and parses the incoming JSON error response
func ReadError(source io.ReadCloser, dest *ErrorBody) error {
	buf, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, dest); err != nil {
		return err
	}

	return nil
}

// CreateRequest converts a struct defined in .proto file to a string JSON.
// If more than one struct is passed, they're converted into new-line delimited JSON for streaming.
func CreateRequest(postData ...proto.Message) (*strings.Reader, error) {
	var m jsonpb.Marshaler
	postJSONStr := ""

	for _, pD := range postData {
		str, err := m.MarshalToString(pD)
		if err != nil {
			return nil, err
		}

		if len(postJSONStr) > 0 {
			postJSONStr += "\n"
		}

		postJSONStr += str
	}

	return strings.NewReader(postJSONStr), nil
}

func TestMain(m *testing.M) {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	dbMySQL, err = mysql.NewDB(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		config("MYSQL_USERNAME"),
		config("MYSQL_PASSWORD"),
		config("MYSQL_HOST"),
		config("MYSQL_PORT"),
		config("MYSQL_DBNAME")))

	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	client, err := mongodb.NewDB(config("MONGODB_URI"))
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	dbMongo = client.Database(config("MONGODB_DBNAME")).Collection("sms")

	os.Exit(m.Run())
}
