package main

import (
	"flag"
	"fmt"
	"networkpolicies/pkg/k8s"
	"os"
)

var (
	help bool

	srcpod string
	dstpod string

	srcnamespace string
	dstnamespace string

	protocol string
	port string

	kubeconfig string
)

func init() {
	flag.BoolVar(&help, "h", false, "this `help`")

	flag.StringVar(&srcnamespace, "sn", "", "specify the `namespace` of source Pod, required")
	flag.StringVar(&dstnamespace, "dn", "", "specify the `namespace` of destination Pod, required")
	flag.StringVar(&srcpod, "s", "", "specify the `source` Pod name, required")
	flag.StringVar(&dstpod, "d", "", "specify the `destination` Pod name, required")
	flag.StringVar(&protocol, "p", "", "specify the `protocol` of the connection")
	flag.StringVar(&port, "port", "", "specify the `port` of the connection")
	flag.StringVar(&kubeconfig, "k", "", "specify kubernetes cluster `kubeconfig` file path, required")

	flag.Usage = usage
}

func main() {

	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}

    //srcPodName := "client"
	////srcPodName := "pod-ingress"
    //dstPodName := "web"
	//
    //srcNamespace := "default"
	////srcNamespace := "other"
    //dstNamespace := "default"
	//
    //protocol := "TCP" // optional, set "" if no need to check
    //port := "6379" // optional, set "" if no need to check
	//
	//kapi, err := k8s.NewKubernetesAPIClient("", "/Users/yikew/antrea-config/admin.conf")

	kapi, err := k8s.NewKubernetesAPIClient("", kubeconfig)
	if err != nil {
		fmt.Errorf("error to get a kubernetes client: %s", err)
	}

	pc, err :=  kapi.PodConnectionAnalysis(srcpod, srcnamespace, dstpod, dstnamespace, protocol, port)
	if err != nil {
		fmt.Printf("fail to get src to dst status: %s", err)
		os.Exit(1)
	}
	k8s.PrintPodConnection(pc)

	//dstTosrc, tip, err :=  kapi.PodConnectionAnalysis(dstPodName, dstNamespace,srcPodName, srcNamespace, protocol, port)
	//if err != nil {
	//	fmt.Errorf("fail to get dst to src status: %s", err)
	//}
	//fmt.Printf("dst Pod to src Pod is allowed %s? %t\n", tip, dstTosrc)
}

func usage() {
	fmt.Fprintf(os.Stderr, `podconnection: a tool to analyze Pod Connection status based on network policies.
Usage:
`)
	flag.PrintDefaults()
}