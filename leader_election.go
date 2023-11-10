package main

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func NewLeaderElection(clientset kubernetes.Interface, id, namespace string) *LeaderElection {
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "lingo.substratus.ai",
			Namespace: namespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	isLeader := &atomic.Bool{}

	config := leaderelection.LeaderElectionConfig{
		Lock: lock,
		// TODO: Set to true after ensuring autoscaling is done before cancel:
		ReleaseOnCancel: false,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Println("Started leading")
				isLeader.Store(true)
			},
			OnStoppedLeading: func() {
				log.Println("Stopped leading")
				isLeader.Store(false)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				log.Printf("New leader elected: %s", identity)
			},
		},
	}

	return &LeaderElection{
		IsLeader: isLeader,
		config:   config,
	}
}

type LeaderElection struct {
	config   leaderelection.LeaderElectionConfig
	IsLeader *atomic.Bool
}

func (le *LeaderElection) Start(ctx context.Context) {
	leaderelection.RunOrDie(ctx, le.config)
}
