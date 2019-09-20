package tracing

import (
	"log"

	datadog "github.com/DataDog/opencensus-go-exporter-datadog"
	"github.com/OmarElGabry/go-callme/internal/pkg/config"

	"contrib.go.opencensus.io/exporter/jaeger"
)

// NewJaegerExporter creates a Jaeger exporter
func NewJaegerExporter(service string) (*jaeger.Exporter, error) {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Couldn't load env variables: %v", err)
	}

	agentEndpointURI := config("TRACING_SERVER_HOST") + ":6831"
	collectorEndpointURI := "http://" + config("TRACING_SERVER_HOST") + ":14268/api/traces"

	je, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint:     agentEndpointURI,
		CollectorEndpoint: collectorEndpointURI,
		ServiceName:       "go-textnow-" + service,
	})

	if err != nil {
		return nil, err
	}

	return je, nil
}

// NewDataDogExporter creates a DataDog exporter
func NewDataDogExporter() (*datadog.Exporter, error) {
	dd, err := datadog.NewExporter(datadog.Options{
		// Need to be configured!
		// TraceAddr: "localhost:8126",
		// StatsAddr: "localhost:8126",
	})

	if err != nil {
		return nil, err
	}

	return dd, nil
}
