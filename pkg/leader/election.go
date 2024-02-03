package leader

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/flowcontrol"
)

func NewElection(clientset kubernetes.Interface, id, namespace string) *Election {
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

	return &Election{
		IsLeader: isLeader,
		config:   config,
	}
}

type Election struct {
	config   leaderelection.LeaderElectionConfig
	IsLeader *atomic.Bool
}

func (le *Election) Start(ctx context.Context) {
	backoff := flowcontrol.NewBackOff(1*time.Second, 15*time.Second)
	const backoffID = "lingo-leader-election"
	retryCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if retryCount > 0 {
				backoff.Next(backoffID, backoff.Clock.Now())
				delay := backoff.Get(backoffID)
				log.Printf("Leader election failed, retrying in %v. RetryCount: %v", delay, retryCount+1)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}
			log.Printf("Starting leader election process. RetryCount: %v", retryCount+1)
			leaderelection.RunOrDie(ctx, le.config)
			retryCount++
		}
	}
}
