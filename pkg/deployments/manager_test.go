package deployments

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rtfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReadinessChecker(t *testing.T) {
	tests := map[string]struct {
		bootstrapped bool
		expectError  bool
	}{
		"not_bootstrapped": {
			expectError: true,
		},
		"bootstrapped": {
			bootstrapped: true,
			expectError:  false,
		},
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			mgr := &Manager{
				Client: rtfake.NewClientBuilder().Build(),
			}
			if spec.bootstrapped {
				require.NoError(t, mgr.Bootstrap(context.TODO()))
			}
			// when
			gotErr := mgr.ReadinessChecker(nil)
			if spec.expectError {

				assert.Error(t, gotErr)
				return
			}
			assert.NoError(t, gotErr)
		})
	}
}

func TestAddDeployment(t *testing.T) {
	specs := map[string]struct {
		deployment    appsv1.Deployment
		expScale      scale
		expectedError error
		expModels     []string
	}{
		"single model - default replica settings": {
			deployment: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-deployment",
					Annotations: map[string]string{
						lingoDomain + "/models": "my-model1",
					},
				},
			},
			expModels: []string{"my-model1"},
			expScale:  scale{Current: 3, Min: 0, Max: 3},
		},
		"single model - annotated": {
			deployment: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-deployment",
					Annotations: map[string]string{
						lingoDomain + "/models":       "my-model1",
						lingoDomain + "/min-replicas": "2",
						lingoDomain + "/max-replicas": "5",
					},
				},
			},
			expModels: []string{"my-model1"},
			expScale:  scale{Current: 3, Min: 2, Max: 5},
		},
		"multi model": {
			deployment: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-deployment",
					Annotations: map[string]string{
						lingoDomain + "/models": "my-model1,my-model2",
					},
				},
			},
			expModels: []string{"my-model1", "my-model2"},
			expScale:  scale{Current: 3, Min: 0, Max: 3},
		},
		"no model - skipped": {
			deployment: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-deployment",
					Annotations: map[string]string{},
				},
			},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			depScale := &autoscalingv1.Scale{
				Spec: autoscalingv1.ScaleSpec{
					Replicas: 3,
				},
			}

			r := &Manager{
				Client:            &partialFakeClient{subRes: depScale},
				Namespace:         "default",
				modelToDeployment: make(map[string]string),
				scalers:           map[string]*scaler{},
			}

			// when
			gotErr := r.addDeployment(context.Background(), spec.deployment)

			// then
			require.NoError(t, gotErr)

			for _, v := range spec.expModels {
				dep, ok := r.ResolveDeployment(v)
				require.True(t, ok)
				assert.Equal(t, "my-deployment", dep)
			}
			assert.Len(t, r.modelToDeployment, len(spec.expModels))
			scales := r.getScalesSnapshot()
			assert.Equal(t, spec.expScale, scales["my-deployment"])
		})
	}
}

func TestScalerState(t *testing.T) {
	const myDeployment = "myDeployment"
	ctx, cancel := context.WithCancel(context.Background())
	r1, r2 := make(chan int32, 1), make(chan int32, 1)

	doReconcile := func(s *scaler, b <-chan int32) {
		for {
			select {
			case n := <-b: // reconcile
				s.UpdateState(n, 0, 3)
			case <-ctx.Done():
				return
			}
		}
	}

	// leader
	s1 := setupTestScaler(t, myDeployment, "old leader", r1, r2)
	// add deployment on reconcile
	s1.UpdateState(1, 0, 3)
	// scale down: idle
	s1.SetDesiredScale(0)
	// reconcile
	// s1.UpdateState(0, 0, 3)
	go doReconcile(s1, r1)

	// new leader instance elected, old one still has state
	s2 := setupTestScaler(t, myDeployment, "new leader", r1, r2)
	// setup on startup
	s2.UpdateState(0, 0, 3)
	// some requests, scale up by autoscaler
	s2.SetDesiredScale(1)
	// s2.UpdateState(1, 0, 3)
	go doReconcile(s2, r2)

	// finalize
	time.Sleep(2 * time.Second)
	cancel()
}

func setupTestScaler(t *testing.T, myDeployment, name string, broadcasts ...chan<- int32) *scaler {
	m1 := &Manager{
		scalers:           make(map[string]*scaler),
		modelToDeployment: make(map[string]string),
	}
	m1.setModelMapping("model1", myDeployment)
	s := m1.getScaler(myDeployment)
	s.scaleDownDelay = 50 * time.Millisecond
	s.scaleFunc = func(n int32, atLeastOne bool) error {
		t.Logf("%s scaler: new replicas: %d, %v", name, n, atLeastOne)
		for _, b := range broadcasts {
			go func(b chan<- int32) {
				time.Sleep(time.Duration(rand.Int()%100) * time.Nanosecond)
				b <- n
			}(b)
		}
		return nil
	}
	return s
}

type partialFakeClient struct {
	client.Client
	subRes client.Object
}

func (f *partialFakeClient) SubResource(subResource string) client.SubResourceClient {
	return &partialSubResFakeClient{sourceSubRes: &f.subRes}
}

type partialSubResFakeClient struct {
	client.SubResourceClient
	sourceSubRes *client.Object
}

func (f *partialSubResFakeClient) Get(ctx context.Context, obj client.Object, target client.Object, opts ...client.SubResourceGetOption) error {
	reflect.ValueOf(target).Elem().Set(reflect.ValueOf(*f.sourceSubRes).Elem())
	return nil
}
