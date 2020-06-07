package k8s

import (
	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)
type k8sClient struct {
	Clientset kubernetes.Interface
}

func NewKubernetesAPIClient(masterUrl string, configPath string) (*k8sClient, error) {
	if configPath == "" {
		return nil, errors.New("no config file provided")
	}

	config, err := clientcmd.BuildConfigFromFlags(masterUrl, configPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kc := &k8sClient{
		Clientset: clientset,
	}

	return kc, nil
}
