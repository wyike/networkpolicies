package k8s

import (
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	return ingressSetGenerator(matchedPolicies)
}

// ListEgressRulesPerPod Generate a set of EgressRules that apply to the pod given in parameter.
// returns nil if the policies in parameters are not applicable to Egress
func (k *k8sClient) ListEgressRulesPerPod(pod *corev1.Pod) (*[]networking.NetworkPolicyEgressRule, error) {
	matchedPolicies, err := k.ListPoliciesPerPod(pod)
	if err != nil {
		return nil, err
	}
	return egressSetGenerator(matchedPolicies)
}

// generate a new table of IngressRules which are the Logical OR of all the existing IngressRules from all the policies given in parameter
// returns nil if the policies in parameters are not applicable to Ingress
func ingressSetGenerator(policies *networking.NetworkPolicyList) (*[]networking.NetworkPolicyIngressRule, error) {
	ingressRules := []networking.NetworkPolicyIngressRule{}

	for _, policy := range policies.Items {
		if IsPolicyApplicableToIngress(&policy) {
			//applicable = true
			for _, singleRule := range policy.Spec.Ingress {
				ingressRules = append(ingressRules, singleRule)
			}
		}
	}

	return &ingressRules, nil
}

// generate a new table of IngressRules which are the Logical OR of all the existing IngressRules from all the policies given in parameter
// returns nil if the policies in parameters are not applicable to Egress
func egressSetGenerator(policies *networking.NetworkPolicyList) (*[]networking.NetworkPolicyEgressRule, error) {
	egressRules := []networking.NetworkPolicyEgressRule{}

	for _, policy := range policies.Items {
		if IsPolicyApplicableToEgress(&policy) {
			for _, singleRule := range policy.Spec.Egress {
				egressRules = append(egressRules, singleRule)
			}
		}
	}

	return &egressRules, nil
}

// IsPolicyApplicableToIngress returns true if the policy is applicable for Ingress traffic
func IsPolicyApplicableToIngress(policy *networking.NetworkPolicy) bool {

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
func IsPolicyApplicableToEgress(policy *networking.NetworkPolicy) bool {

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