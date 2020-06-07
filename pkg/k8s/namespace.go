package k8s

import (
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *k8sClient) GetNamespaceByName(nspName string) (*corev1.Namespace, error) {
	ns, err := k.Clientset.CoreV1().Namespaces().Get(nspName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "fail to get a namespace")
	}

	return ns, nil
}
