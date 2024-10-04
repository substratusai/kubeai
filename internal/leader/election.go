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

func NewElection(clientset kubernetes.Interface, id, namespace string,
	leaseDuration time.Duration,
	renewDeadline time.Duration,
	retryPeriod time.Duration,
) *Election {
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "kubeai.org",
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
		LeaseDuration:   leaseDuration,
		RenewDeadline:   renewDeadline,
		RetryPeriod:     retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Printf("%q started leading", id)
				isLeader.Store(true)
			},
			OnStoppedLeading: func() {
				log.Printf("%q stopped leading", id)
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
		ID:       id,
	}
}

type Election struct {
	config   leaderelection.LeaderElectionConfig
	IsLeader *atomic.Bool
	ID       string
}

func (le *Election) Start(ctx context.Context) error {
	backoff := flowcontrol.NewBackOff(1*time.Second, 15*time.Second)
	const backoffID = "kubeai-leader-election"
	for {
		leaderelection.RunOrDie(ctx, le.config)
		backoff.Next(backoffID, backoff.Clock.Now())
		delay := backoff.Get(backoffID)
		log.Printf("Leader election stopped on %q, retrying in %s", le.ID, delay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
