package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/OmarElGabry/go-textnow/internal/pkg/redis"

	// mysql driver
	"github.com/OmarElGabry/go-textnow/internal/phonebook"
	"github.com/OmarElGabry/go-textnow/internal/pkg/config"
	"github.com/OmarElGabry/go-textnow/internal/pkg/validator"
	_ "github.com/go-sql-driver/mysql"

	"google.golang.org/grpc"

	"github.com/OmarElGabry/go-textnow/internal/pkg/mysql"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	// connect to mysql database
	db, err := mysql.NewDB(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		config("MYSQL_USERNAME"),
		config("MYSQL_PASSWORD"),
		config("MYSQL_HOST"),
		config("MYSQL_PORT"),
		config("MYSQL_DBNAME")))

	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	// connect to redis cache
	cache, err := redis.NewCache()

	// metrics and tracing
	// 	jaeger only supports tracing
	// je, err := tracing.NewJaegerExporter("phonebook")
	// if err != nil {
	// 	log.Fatalf("Failed to create the Jaeger exporter: %v", err)
	// }
	// defer je.Flush()

	// trace.RegisterExporter(je)
	// trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	// spin up the gRPC server
	lis, err := net.Listen("tcp", ":"+config("GRPC_SERVER_PORT"))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// create new server and register metrics and tracing handler
	opts := []grpc.ServerOption{ /*grpc.StatsHandler(&ocgrpc.ServerHandler{})*/ }
	opts = append(opts, validator.Middlewares()...)

	s := grpc.NewServer(opts...)
	srv := phonebook.NewPhoneBookServiceServer(db, cache)
	phonebook.RegisterPhoneBookServiceServer(s, srv)

	// graceful shutdown
	c := make(chan os.Signal, 1)

	go func() {
		log.Println("Phonebook listening ...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	<-c
	s.Stop()
	lis.Close()
	db.Close()
}
