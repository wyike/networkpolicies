package k8s

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PodConnectionAnalysis gives if srcPod can reach dstPod by
// checking if srcPod in dstPod allowed ingress rules or dstPod in srcPod egress rules
func (k *k8sClient) PodConnectionAnalysis(srcPodName, srcNsName, dstPodName, podBNamespace, protocol, port string) (bool, string, error) {

	srcPod, err := k.GetPodByNameInNamespace(srcNsName, srcPodName)
	if err != nil {
		return false, "", errors.Wrap(err, "fail to get src Pod")
	}

	dstPod, err := k.GetPodByNameInNamespace(podBNamespace, dstPodName)
	if err != nil {
		return false, "", errors.Wrap(err, "fail to get dst Pod")
	}

	srcPodIngressRules, err := k.ListEgressRulesPerPod(srcPod)
	if err != nil {
		return false, "", errors.Wrap(err, "fail to get Ingress rules applied on src Pod")
	}

	dstPodIngressRules, err := k.ListIngressRulesPerPod(dstPod)
	if err != nil {
		return false, "", errors.Wrap(err, "fail to get Ingress rules applied on dst Pod")
	}

	for _, ingressRule := range *dstPodIngressRules {
		allow, err := k.IsPodInNetworkPolicyPeer(srcPod, ingressRule.From, dstPod.Namespace)
		if err != nil {
			return false, "", errors.Wrap(err, "fail to check if src Pod in dst Pod ingress rules")
		}

		tipmsg, err := IsProtocolPortInNetworkPolicyPorts(protocol, port, ingressRule.Ports)
		if err != nil {
			return false, "", errors.Wrap(err,"fail to get protocol:port message")
		}

		if allow {
			return true, tipmsg, nil
		}
	}

	for _, egressRule := range *srcPodIngressRules {
		allow, err := k.IsPodInNetworkPolicyPeer(srcPod, egressRule.To, srcPod.Namespace)
		if err != nil {
			return false, "", errors.Wrap(err, "fail to check if dst Pod in src Pod egress rules")
		}

		tipmsg, err := IsProtocolPortInNetworkPolicyPorts(protocol, port, egressRule.Ports)
		if err != nil {
			return false, "", errors.Wrap(err,"fail to get protocol:port message")
		}

		if allow {
			return true, tipmsg, nil
		}
	}

	return false, "", nil
}

// IsPodInNetworkPolicyPeer checks if a Pod in an ingress/egress rule network policy peers list
// policyns is the namespace which the ingress/egress rule stays in
func (k *k8sClient) IsPodInNetworkPolicyPeer(pod *corev1.Pod, peers []networking.NetworkPolicyPeer, policyns string) (bool, error) {

	podLabels := labels.Set(pod.GetLabels())
	ns, _:= k.GetNamespaceByName(pod.Namespace)
	nsLabels:= labels.Set(ns.GetLabels())

	for _, peer := range peers {
		var podselector labels.Selector
		var nsselector labels.Selector
		podselector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return false, err
		}
		nsselector, err = metav1.LabelSelectorAsSelector(peer.NamespaceSelector)
		if err != nil {
			return false, err
		}
		// TODO: go through kubernetes source code, see how they handle different cases
		switch {
		case peer.PodSelector != nil && peer.NamespaceSelector != nil:
			if podselector.Matches(podLabels) && nsselector.Matches(nsLabels) {
				return true, nil
			}
		case peer.PodSelector != nil && peer.NamespaceSelector == nil:
			if podselector.Matches(podLabels) && pod.Namespace == policyns {
				return true, nil
			}
		case peer.PodSelector == nil && peer.NamespaceSelector != nil:
			podselector = labels.Everything()
			if nsselector.Matches(nsLabels) {
				return true, nil
			}
		default:
			fmt.Println("Unhandled case: block by default")
			continue
		}

		// IPBlock analysis is meaningless. These should be cluster-external IPs,
		// since Pod IPs are ephemeral and unpredictable
	}

	return false, errors.New("No matched entities, blocked by default")
}

// IsProtocolPortInNetworkPolicyPorts checks if protocol:port in an ingress/egress rule ports list
func IsProtocolPortInNetworkPolicyPorts(protocol, port string, ports []networking.NetworkPolicyPort) (string, error) {

	switch {
	case protocol == "" && port == "":
		return "", nil
	case protocol != "" && port != "":
		for _, npport := range ports {
			if *npport.Protocol == corev1.Protocol(protocol) &&
				(npport.Port.StrVal == port || strconv.FormatInt(int64(npport.Port.IntVal), 10) == port) {
				return fmt.Sprintf("on %s port %s",protocol, port), nil
			}
		}
	default:
		return "", errors.New("Please provide protocol and port pair")
	}

	return "", nil
}