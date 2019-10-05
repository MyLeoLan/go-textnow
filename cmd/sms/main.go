package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/OmarElGabry/go-textnow/internal/phonebook"
	"github.com/OmarElGabry/go-textnow/internal/pkg/config"
	"github.com/OmarElGabry/go-textnow/internal/pkg/mongodb"
	"github.com/OmarElGabry/go-textnow/internal/pkg/validator"

	"github.com/OmarElGabry/go-textnow/internal/sms"
	"google.golang.org/grpc"

	"go.opencensus.io/plugin/ocgrpc"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	// connect to mongodb database
	client, err := mongodb.NewDB(config("MONGODB_URI"))
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	dbCollection := client.Database(config("MONGODB_DBNAME")).Collection("sms")

	// connect to phonebook server
	// and register metrics and tracing handler
	conn, err := grpc.Dial("phonebook-service:"+config("GRPC_SERVER_PORT"),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect to phonebook server from sms: %v", err)
	}

	pB := phonebook.NewPhoneBookServiceClient(conn)

	// metrics and tracing
	// 	jaeger only supports tracing
	// je, err := tracing.NewJaegerExporter("sms")
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
	// make sure to put stats handler first
	opts := []grpc.ServerOption{ /*grpc.StatsHandler(&ocgrpc.ServerHandler{})*/ }
	opts = append(opts, validator.Middlewares()...)

	s := grpc.NewServer(opts...)
	srv := sms.NewSMSServiceServer(dbCollection, pB)
	sms.RegisterSMSServiceServer(s, srv)

	// graceful shutdown
	c := make(chan os.Signal, 1)

	go func() {
		log.Println("SMS listening ...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	<-c
	log.Println("SMS server stoped! ...")
	s.Stop()
	lis.Close()
	client.Disconnect(context.TODO())
}
