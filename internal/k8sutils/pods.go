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
func PodHash(podSpec corev1.PodSpec, extraData ...string) string {
	podTemplateSpecHasher := fnv.New32a()
	DeepHashObject(podTemplateSpecHasher, podSpec)

	// Add any extra data to the hash calculation
	for _, data := range extraData {
		if data != "" {
			podTemplateSpecHasher.Write([]byte(data))
		}
	}

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

func ContainerIsReady(pod *corev1.Pod, containerName string) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return status.Ready
		}
	}
	return false
}
