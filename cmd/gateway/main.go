package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/OmarElGabry/go-callme/internal/pkg/config"

	"google.golang.org/grpc"

	"github.com/OmarElGabry/go-callme/internal/phonebook"
	"github.com/OmarElGabry/go-callme/internal/sms"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	// register phonebook and sms services
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	// phonebook
	err = phonebook.RegisterPhoneBookServiceHandlerFromEndpoint(ctx, mux,
		"phonebook:"+config("GRPC_SERVER_PORT"), opts)
	if err != nil {
		log.Fatalf("gateway: failed to register phonebook service: %v", err)
	}

	// sms
	err = sms.RegisterSMSServiceHandlerFromEndpoint(ctx, mux,
		"sms:"+config("GRPC_SERVER_PORT"), opts)
	if err != nil {
		log.Fatalf("gateway: failed to register sms service: %v", err)
	}

	s := &http.Server{Addr: ":8080", Handler: mux}

	// graceful shutdown
	c := make(chan os.Signal, 1)

	go func() {
		log.Println("Gateway listening ...")
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("gateway: failed to listen: %v", err)
		}
	}()

	<-c
	s.Shutdown(ctx)
}
