package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"k8s.io/client-go/util/flowcontrol"
)

func main() {
	// Cancel context on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}

func Run(ctx context.Context) error {
	backoff := flowcontrol.NewBackOff(1*time.Second, 15*time.Second)
	const backoffID = "lingo-leader-election"

	for {
		// This loop is needed because leaderelection.RunOrDie() exits when
		// it looses leadership... This can be an issue if Lingo looses connection
		// to the Kubernetes API Server.
		RunOrDie(ctx)

		// Calculate the next backoff duration.
		backoff.Next(backoffID, backoff.Clock.Now())
		wait := backoff.Get(backoffID)
		log.Println("waiting", wait)

		// Wait for context cancellation or the backoff duration.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			continue
		}
	}
}

func RunOrDie(ctx context.Context) {
	// Simulating RunOrDie from leaderelection...

	log.Println("runOrDie() called")
	defer log.Println("runOrDie() returned")

	// Wait for context cancellation or a second to go by.
	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
	}
}
