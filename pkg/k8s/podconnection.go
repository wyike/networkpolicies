package k8s

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type PodConnection struct  {
	src *corev1.Pod
	dst *corev1.Pod
	protocol string
	port string
	reachable bool
	reachableOnPort bool
	ruleAllows string
	policyAllows *networking.NetworkPolicy
	srcpolicies *networking.NetworkPolicyList
	dstpolicies *networking.NetworkPolicyList
}

//// PodConnectionAnalysis gives if srcPod can reach dstPod by
//// checking if srcPod in dstPod allowed ingress rules or dstPod in srcPod egress rules
func (k *k8sClient) PodConnectionAnalysis(srcPodName, srcNsName, dstPodName, podBNamespace, protocol, port string) (PodConnection, error) {

	srcPod, err := k.GetPodByNameInNamespace(srcNsName, srcPodName)
	if err != nil {
		return PodConnection{}, errors.Wrap(err, "fail to get src Pod")
	}

	dstPod, err := k.GetPodByNameInNamespace(podBNamespace, dstPodName)
	if err != nil {
		return PodConnection{}, errors.Wrap(err, "fail to get dst Pod")
	}

	srcAppliedPolicies, err := k.ListPoliciesPerPod(srcPod)
	if err != nil {
		return PodConnection{}, errors.Wrap(err, "fail to get policies applied on src Pod")
	}

	dstAppliedPolicies, err := k.ListPoliciesPerPod(dstPod)
	if err != nil{
		return PodConnection{}, errors.Wrap(err, "fail to get policies applied on dst Pod")
	}

	pc := PodConnection{
		src:             srcPod,
		dst:             dstPod,
		protocol:        protocol,
		port:            port,
		srcpolicies:     srcAppliedPolicies,
		dstpolicies:     dstAppliedPolicies,
	}

	var srcHasPolicy, srcHasEgress bool
	for _, policy := range srcAppliedPolicies.Items {
		srcHasPolicy = true
		if k.IsPolicyApplicableToEgress(&policy) {
			srcHasEgress = true
			for _, singleRule := range policy.Spec.Egress {
				pc.reachable, pc.ruleAllows, err = k.IsPodInNetworkPolicyPeer(srcPod, singleRule.To, srcPod.Namespace)
				if err != nil {
					return PodConnection{}, errors.Wrap(err, "fail to check if dst Pod in src Pod egress rules")
				}

				pc.reachableOnPort, err = IsProtocolPortInNetworkPolicyPorts(protocol, port, singleRule.Ports)
				if err != nil {
					return PodConnection{}, errors.Wrap(err,"fail to get protocol:port message")
				}

				if pc.reachable {
					pc.policyAllows = &policy
					return pc, nil
				}
			}
		}
	}

	var dstHasPolicy, dstHasIngress bool
	for _, policy := range dstAppliedPolicies.Items {
		dstHasPolicy = true
		if k.IsPolicyApplicableToIngress(&policy) {
			dstHasIngress = true
			for _, singleRule := range policy.Spec.Ingress {
				pc.reachable, pc.ruleAllows, err = k.IsPodInNetworkPolicyPeer(srcPod, singleRule.From, dstPod.Namespace)
				if err != nil {
					return PodConnection{}, errors.Wrap(err, "fail to check if src Pod in dst Pod ingress rules")
				}

				pc.reachableOnPort, err = IsProtocolPortInNetworkPolicyPorts(protocol, port, singleRule.Ports)
				if err != nil {
					return PodConnection{}, errors.Wrap(err,"fail to get protocol:port message")
				}

				if pc.reachable {
					pc.policyAllows = &policy
					return pc, nil
				}
			}
		}
	}

	// When no policies on srcPod and dstPod
	// When dst pod only has egress rules
	// When src pod only has ingress rules
	// Allow by default
	if (!srcHasPolicy || !srcHasEgress) && (!dstHasPolicy || !dstHasIngress) {
		pc.reachable = true
		pc.reachableOnPort = true
		return pc, nil
	}

	// When src to dst is blocking, set the policies applied on Pods
	return pc, nil
}

// IsPodInNetworkPolicyPeer checks if a Pod in an ingress/egress rule network policy peers list
// policyns is the namespace which the ingress/egress rule stays in
func (k *k8sClient) IsPodInNetworkPolicyPeer(pod *corev1.Pod, peers []networking.NetworkPolicyPeer, policyns string) (bool, string, error) {

	podLabels := labels.Set(pod.GetLabels())
	ns, _:= k.GetNamespaceByName(pod.Namespace)
	nsLabels:= labels.Set(ns.GetLabels())

	for _, peer := range peers {
		var podselector labels.Selector
		var nsselector labels.Selector
		podselector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return false, "", err
		}
		nsselector, err = metav1.LabelSelectorAsSelector(peer.NamespaceSelector)
		if err != nil {
			return false, "", err
		}

		var ruleHuman string
		// TODO: go through kubernetes source code, see how they handle different cases
		switch {
		case peer.PodSelector != nil && peer.NamespaceSelector != nil:
			if podselector.Matches(podLabels) && nsselector.Matches(nsLabels) {
				//podHuman = podLabels.String()+"/"+nsLabels.String()
				ruleHuman = podselector.String()+"/"+nsselector.String()
				return true, "PodSelector/NamespaceSelector:" + ruleHuman, nil
			}
		case peer.PodSelector != nil && peer.NamespaceSelector == nil:
			if podselector.Matches(podLabels) && pod.Namespace == policyns {
				//podHuman = podLabels.String()
				ruleHuman = podselector.String()
				return true, fmt.Sprintf("PodSelector:%s", ruleHuman), nil
			}
		case peer.PodSelector == nil && peer.NamespaceSelector != nil:
			podselector = labels.Everything()
			if nsselector.Matches(nsLabels) {
				//podHuman = podselector.String()
				// need test
				ruleHuman = nsselector.String()
				return true, "NamespaceSelector:" + ruleHuman, nil
			}
		default:
			fmt.Println("Unhandled case: block by default")
			continue
		}

		// IPBlock analysis is meaningless. These should be cluster-external IPs,
		// since Pod IPs are ephemeral and unpredictable
	}

	return false, "", nil
}

// IsProtocolPortInNetworkPolicyPorts checks if protocol:port in an ingress/egress rule ports list
func IsProtocolPortInNetworkPolicyPorts(protocol, port string, ports []networking.NetworkPolicyPort) (bool, error) {

	switch {
	case protocol == "" && port == "":
		return true, nil
	case protocol != "" && port != "":
		for _, npport := range ports {
			if *npport.Protocol == corev1.Protocol(protocol) &&
				(npport.Port.StrVal == port || strconv.FormatInt(int64(npport.Port.IntVal), 10) == port) {
				return true, nil
			}
		}
	default:
		return false, errors.New("Please provide protocol and port pair")
	}

	return false, nil
}

func printPolicies(policies *networking.NetworkPolicyList) {
	for count, policy := range policies.Items {
		fmt.Printf("POLICY %d\n", count+1)
		pp, _ := json.MarshalIndent(&policy, "", "   ")
		fmt.Println(string(pp))
	}
}

func PrintPodConnection(pc PodConnection) {
	if pc.reachable {
		if pc.protocol == "" && pc.port == "" {
			fmt.Printf("[Pod:%s]-[Namespace:%s] to [Pod:%s]-[Namespace:%s] is reachable\n", pc.src.Name, pc.src.Namespace, pc.dst.Name, pc.dst.Namespace)
		}
		if pc.protocol != "" && pc.port != "" && pc.reachableOnPort {
			fmt.Printf("[Pod:%s]-[Namespace:%s] to [Pod:%s]-[Namespace:%s] is reachable on %s:%s\n", pc.src.Name, pc.src.Namespace, pc.dst.Name, pc.dst.Namespace, pc.protocol, pc.port)
		}
		if pc.protocol != "" && pc.port != "" && !pc.reachableOnPort {
			fmt.Printf("[Pod:%s]-[Namespace:%s] to [Pod:%s]-[Namespace:%s] is not reachable on %s:%s\n", pc.src.Name, pc.src.Namespace, pc.dst.Name, pc.dst.Namespace, pc.protocol, pc.port)
		}
		if pc.policyAllows != nil {
			fmt.Printf("Reason: [Rule:%s]-[Policy:%s]-[Namespace:%s] allows the connection\n", pc.ruleAllows, pc.policyAllows.Name, pc.policyAllows.Namespace)
		} else {
			fmt.Printf("Reason: No policy rule working on the connection, allows the connection by default\n")
		}
	}

	if !pc.reachable {
		fmt.Printf("[Pod:%s]-[Namespace:%s] to [Pod:%s]-[Namespace:%s] is not reachable\n", pc.src.Name, pc.src.Namespace, pc.dst.Name, pc.dst.Namespace)
		if len(pc.srcpolicies.Items) != 0 {
			srcpolicienames := []string{}
			for _, policy := range pc.srcpolicies.Items {
				srcpolicienames = append(srcpolicienames, policy.Name)
			}
			fmt.Printf("Reason: [Policy:%s]-[Namespace:%s] on [Pod:%s]-[Namespace:%s] is blocking the connection\n", strings.Join(srcpolicienames[:], ","), pc.src.Namespace, pc.src.Name, pc.src.Namespace)
		}

		if len(pc.dstpolicies.Items) != 0 {
			dstpolicienames := []string{}
			for _, policy := range pc.dstpolicies.Items {
				dstpolicienames = append(dstpolicienames, policy.Name)
			}
			fmt.Printf("Reason: [Policy:%s]-[Namespace:%s] on [Pod:%s]-[Namespace:%s] is blocking the connection\n", strings.Join(dstpolicienames[:], ","), pc.dst.Namespace, pc.dst.Name, pc.dst.Namespace)
		}
	}

}