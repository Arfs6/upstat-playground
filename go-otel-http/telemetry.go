package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupTelemetry(ctx context.Context) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New() // create an exporter.
	if err != nil {
		return nil, err
	}

	// Describe our application.
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "go-otel-http"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create the tracer provider.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register the provider globally.
	otel.SetTracerProvider(tp)

	// Configure trace context propagation
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}
