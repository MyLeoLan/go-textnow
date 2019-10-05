package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/OmarElGabry/go-textnow/internal/pkg/config"

	"google.golang.org/grpc"

	"github.com/OmarElGabry/go-textnow/internal/phonebook"
	"github.com/OmarElGabry/go-textnow/internal/sms"

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
		"phonebook-service:"+config("GRPC_SERVER_PORT"), opts)
	if err != nil {
		log.Fatalf("gateway: failed to register phonebook service: %v", err)
	}

	// sms
	err = sms.RegisterSMSServiceHandlerFromEndpoint(ctx, mux,
		"sms-service:"+config("GRPC_SERVER_PORT"), opts)
	if err != nil {
		log.Fatalf("gateway: failed to register sms service: %v", err)
	}

	// add default route "/" required by k8s for health checks
	// @see https://cloud.google.com/kubernetes-engine/docs/concepts/ingress#health_checks
	gatewaymux := http.NewServeMux()
	gatewaymux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "Not found"
		}

		io.WriteString(w, "hello from the gateway: "+hostname)
	})

	// wrap the grpc mux
	gatewaymux.Handle("/", mux)

	s := &http.Server{Addr: ":8080", Handler: gatewaymux}

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
