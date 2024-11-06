package k8sutils

import (
	"fmt"
	"hash"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"
)

func PodIsScheduled(pod *corev1.Pod) bool {
	return pod.Spec.NodeName != ""
}

func PodIsReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// PodHash returns a hash value calculated from Pod spec.
// Inspired by k8s.io/kubernetes/pkg/controller.ComputeHash()
func PodHash(podSpec corev1.PodSpec) string {
	podTemplateSpecHasher := fnv.New32a()
	DeepHashObject(podTemplateSpecHasher, podSpec)

	// TODO: Implement collision detection if needed.
	//// Add collisionCount in the hash if it exists.
	//if collisionCount != nil {
	//	collisionCountBytes := make([]byte, 8)
	//	binary.LittleEndian.PutUint32(collisionCountBytes, uint32(*collisionCount))
	//	podTemplateSpecHasher.Write(collisionCountBytes)
	//}

	return rand.SafeEncodeString(fmt.Sprint(podTemplateSpecHasher.Sum32()))
}

// StringHash returns a hash value calculated from the input string.
func StringHash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return rand.SafeEncodeString(fmt.Sprint(h.Sum32()))
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
// Copied from k8s.io/kubernetes/pkg/util/hash to avoid dependency on k8s.io/kubernetes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	fmt.Fprintf(hasher, "%v", dump.ForHash(objectToWrite))
}

// RemoveEphemeralContainer removes the container with the given name from the pod
// and returns true if it did.
func RemoveEphemeralContainer(pod *corev1.Pod, name string) bool {
	if i := FindEphemeralContainer(pod, name); i != -1 {
		pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers[:i], pod.Spec.EphemeralContainers[i+1:]...)
		return true
	}
	return false
}

// AddEphemeralContainer adds the container to the pod and returns true if it did.
func AddEphemeralContainer(pod *corev1.Pod, container corev1.EphemeralContainer) bool {
	if FindEphemeralContainer(pod, container.Name) != -1 {
		return false
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, container)
	return true
}

// FindEphemeralContainer returns the index of the container with the given name in the pod.
func FindEphemeralContainer(pod *corev1.Pod, name string) int {
	for i, container := range pod.Spec.EphemeralContainers {
		if container.Name == name {
			return i
		}
	}
	return -1
}
