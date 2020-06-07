package k8s

import (
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *k8sClient) ListPodsInNamespace(nspName string) (*corev1.PodList, error) {
	pods, err := k.Clientset.CoreV1().Pods(nspName).List(metav1.ListOptions{})

	return pods, err
}

func (k *k8sClient) GetPodByNameInNamespace(nspName string, podName string) (*corev1.Pod, error) {
	pod, err := k.Clientset.CoreV1().Pods(nspName).Get(podName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "fail to get a Pod")
	}

	return pod, nil
}
