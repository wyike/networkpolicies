package k8s

import (
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// ErrKeyNotFound is returned when entity with provided key is not found in the store
	ErrPolicyNotFound = errors.New("network policy not found")
	// ErrKeyNotUnique is returned when entity with provided key already exists in the store
	ErrIngressNotSet = errors.New("ingress is not set")
	ErrEgressNotSet = errors.New("egress traffic is not set")
)

func (k *k8sClient) ListPoliciesInNamespace(nspName string) (*networking.NetworkPolicyList, error) {
     policies, err := k.Clientset.NetworkingV1().NetworkPolicies(nspName).List(metav1.ListOptions{})
     if err != nil {
     	return nil, err
	 }

     return policies, nil
}

func (k *k8sClient) ListPoliciesPerPod(pod *corev1.Pod) (*networking.NetworkPolicyList, error) {
	appliedPolicies := networking.NetworkPolicyList{
		Items: []networking.NetworkPolicy{},
	}

	policies, _ := k.ListPoliciesInNamespace(pod.Namespace)

	podLabels := labels.Set(pod.GetLabels())
	for _, policy := range policies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil {
			return nil, err
		}
		if selector.Matches(podLabels) {
			appliedPolicies.Items = append(appliedPolicies.Items, policy)
		}
	}

	return &appliedPolicies, nil
}


// ListIngressRulesPerPod Generate a set of IngressRules that apply to the pod given in parameter.
// returns nil if the policies in parameters are not applicable to Ingress
func (k *k8sClient) ListIngressRulesPerPod(pod *corev1.Pod) (*[]networking.NetworkPolicyIngressRule, error) {
	matchedPolicies, err := k.ListPoliciesPerPod(pod)
	if err != nil {
		return nil, err
	}
	return k.ingressSetGenerator(matchedPolicies)
}

// ListEgressRulesPerPod Generate a set of EgressRules that apply to the pod given in parameter.
// returns nil if the policies in parameters are not applicable to Egress
func (k *k8sClient) ListEgressRulesPerPod(pod *corev1.Pod) (*[]networking.NetworkPolicyEgressRule, error) {
	matchedPolicies, err := k.ListPoliciesPerPod(pod)
	if err != nil {
		return nil, err
	}
	return k.egressSetGenerator(matchedPolicies)
}

// generate a new table of IngressRules which are the Logical OR of all the existing IngressRules from all the policies given in parameter
// returns nil if the policies in parameters are not applicable to Ingress
func (k *k8sClient) ingressSetGenerator(policies *networking.NetworkPolicyList) (*[]networking.NetworkPolicyIngressRule, error) {
	ingressRules := []networking.NetworkPolicyIngressRule{}

	var hasPolicy, hasIngress bool
	for _, policy := range policies.Items {
		hasPolicy = true
		if k.IsPolicyApplicableToIngress(&policy) {
			hasIngress = true
			for _, singleRule := range policy.Spec.Ingress {
				ingressRules = append(ingressRules, singleRule)
			}
		}
	}

	var err error
	if !hasPolicy {
		err = ErrPolicyNotFound
	}
	if hasPolicy && !hasIngress {
		err = ErrIngressNotSet
	}

	return &ingressRules, err
}

// generate a new table of IngressRules which are the Logical OR of all the existing IngressRules from all the policies given in parameter
// returns nil if the policies in parameters are not applicable to Egress
func (k *k8sClient) egressSetGenerator(policies *networking.NetworkPolicyList) (*[]networking.NetworkPolicyEgressRule, error) {
	egressRules := []networking.NetworkPolicyEgressRule{}

	var hasPolicy, hasEgress bool
	for _, policy := range policies.Items {
		hasPolicy = true
		if k.IsPolicyApplicableToEgress(&policy) {
			hasEgress = true
			for _, singleRule := range policy.Spec.Egress {
				egressRules = append(egressRules, singleRule)
			}
		}
	}

	var err error
	if !hasPolicy {
		err = ErrPolicyNotFound
	}
	if hasPolicy && !hasEgress {
		err = ErrEgressNotSet
	}

	return &egressRules, err
}

// IsPolicyApplicableToIngress returns true if the policy is applicable for Ingress traffic
func (k *k8sClient) IsPolicyApplicableToIngress(policy *networking.NetworkPolicy) bool {

	// Logic: Policy applies to ingress only IF:
	// - flag is not set
	// - flag is set with an entry to type Ingress (even if no section Egress exists)

	if policy.Spec.PolicyTypes == nil {
		return true
	}

	for _, ptype := range policy.Spec.PolicyTypes {
		if ptype == networking.PolicyTypeIngress {
			return true
		}
	}

	return false
}

// IsPolicyApplicableToEgress returns true if the policy is applicable for Egress traffic
func (k *k8sClient) IsPolicyApplicableToEgress(policy *networking.NetworkPolicy) bool {

	// Logic: Policy applies to egress only IF:
	// - flag is not set but egress section is present
	// - flag is set with an entry to type Egress (even if no section Egress exists)

	if policy.Spec.PolicyTypes == nil {
		if policy.Spec.Egress != nil {
			return true
		}
		return false
	}

	for _, ptype := range policy.Spec.PolicyTypes {
		if ptype == networking.PolicyTypeEgress {
			return true
		}
	}

	return false
}