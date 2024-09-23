package k8sutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSetLabel(t *testing.T) {
	t.Parallel()
	const (
		testKey = "test-key"
		testVal = "test-val"
	)
	cases := []struct {
		name     string
		input    client.Object
		expected client.Object
	}{
		{
			name:     "nil labels",
			input:    &corev1.Pod{},
			expected: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{testKey: testVal}}},
		},
		{
			name:     "existing labels",
			input:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"existing-key": "existing-val", testKey: testVal}}},
			expected: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"existing-key": "existing-val", testKey: testVal}}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SetLabel(c.input, testKey, testVal)
			assert.Equal(t, c.expected, c.input)
		})
	}
}

func TestGetLabel(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"test-key": "test-val"}}}
	assert.Equal(t, "test-val", GetLabel(pod, "test-key"))
}

func TestSetAnnotation(t *testing.T) {
	t.Parallel()
	const (
		testKey = "test-key"
		testVal = "test-val"
	)
	cases := []struct {
		name     string
		input    client.Object
		expected client.Object
	}{
		{
			name:     "nil annotations",
			input:    &corev1.Pod{},
			expected: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{testKey: testVal}}},
		},
		{
			name:     "existing annotations",
			input:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"existing-key": "existing-val", testKey: testVal}}},
			expected: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"existing-key": "existing-val", testKey: testVal}}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SetAnnotation(c.input, testKey, testVal)
			assert.Equal(t, c.expected, c.input)
		})
	}
}
func TestGetAnnotation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"test-key": "test-val"}}}
	assert.Equal(t, "test-val", GetAnnotation(pod, "test-key"))
}
