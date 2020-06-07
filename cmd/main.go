package main

import (
	"fmt"
	"networkpolicies/pkg/k8s"
)
func main() {

    srcPodName := "client"
    dstPodName := "web"

    srcNamespace := "default"
    dstNamespace := "default"

    protocol := "TCP" // optional, set "" if no need to check
    port := "6379" // optional, set "" if no need to check

	kapi, err := k8s.NewKubernetesAPIClient("", "/Users/yikew/antrea-config/admin.conf")
	if err != nil {
		fmt.Errorf("error to get a kubernetes client: %s", err)
	}

	srcTodst, tip, err :=  kapi.PodConnectionAnalysis(srcPodName, srcNamespace, dstPodName, dstNamespace, protocol, port)
	if err != nil {
		fmt.Errorf("fail to get src to dst status: %s", err)
	}
	fmt.Printf("src Pod to dst Pod is allowed %s? %t\n", tip, srcTodst)

	dstTosrc, tip, err :=  kapi.PodConnectionAnalysis(dstPodName, dstNamespace,srcPodName, srcNamespace, protocol, port)
	if err != nil {
		fmt.Errorf("fail to get dst to src status: %s", err)
	}
	fmt.Printf("dst Pod to src Pod is allowed %s? %t\n", tip, dstTosrc)
}