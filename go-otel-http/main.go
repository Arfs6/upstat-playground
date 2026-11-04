package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Listen for ctrl+c.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	// Initialize open telemetry.
	tp, err := setupTelemetry(ctx)
	if err != nil {
		return err
	}

	// Shutdown open telemetry and flush pending spans
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Printf("Telemetry shutdown error: %v", err)
		}
	}()

	// Create an instrumented http client.
	client := &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
		),
		Timeout: 5 * time.Second,
	}

	// Register http roots.
	mux := http.NewServeMux()

	mux.Handle(
		"/hello",
		otelhttp.NewHandler(
			http.HandlerFunc(helloHandler),
			"GET /hello",
		),
	)

	mux.Handle(
		"/proxy",
		otelhttp.NewHandler(
			proxyHandler(client),
			"GET /proxy",
		),
	)

	// Create the http server.
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start the server in a go routine.
	serverErr := make(chan error, 1)
	go func() {
		log.Println("Server running on :8080")
		serverErr <- srv.ListenAndServe()
	}()

	// Wait for an error or ctrl+c.
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}

	case <-ctx.Done():
		log.Println("Shutting down server...")
	}

	// Gracefully shutdown the http server.
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

// Handle GET /hello.
func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello from Go!")
}

// Handel GET /proxy.
func proxyHandler(client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create an outgoing request.
		req, err := http.NewRequestWithContext(
			r.Context(),
			http.MethodGet,
			"http://localhost:8080/hello",
			nil,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Send the request.
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set(
			"Content-Type",
			resp.Header.Get("Content-Type"),
		)
		w.WriteHeader(resp.StatusCode)

		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("Response copy failed: %v", err)
		}
	}
}
