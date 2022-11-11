package main

import (
	"context"
	"flag"
	"fmt"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config.yaml"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()
	ctx := context.TODO()
	//config, err := rest.InClusterConfig()
	//if err != nil {
	/*	fmt.Errorf("error: Getting In CLuster Config: %s", err.Error())
		os.Exit(1)
	}*/
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		fmt.Errorf("error: creating ClientSet: %s", err.Error())
		os.Exit(1)
	}

	for {

		HairPinProxy(clientset, ctx)

		time.Sleep(time.Second * 15)
	}
}

func HairPinProxy(clientset *kubernetes.Clientset, ctx context.Context) {
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Errorf("error: Getting Ingresses Error: %s", err.Error())
		return
	}
	var TLSHosts []networkingv1.IngressTLS
	var Hosts []string
	var dnsConfig strings.Builder
	for _, ingress := range ingresses.Items {
		if len(ingress.Spec.TLS) != 0 {
			TLSHosts = append(TLSHosts, ingress.Spec.TLS...)
		}
	}
	for _, TLSHost := range TLSHosts {
		//TLSHost = TLSHosts[0]
		Hosts = append(Hosts, TLSHost.Hosts...)
	}
	dnsConfig.WriteString("|-\n")
	for _, Host := range Hosts {
		dnsConfig.WriteString(fmt.Sprintf("rewrite name %s hairpin-proxy.hairpin-proxy.svc.cluster.local\n", Host))
	}
	corednscustom, err := clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns-custom", metav1.GetOptions{})
	if err != nil {
		fmt.Errorf("error: Getting ConfigMap: %s", err.Error())
		return
	}
	corednscustom.Data["changeme.override"] = dnsConfig.String()
	_, err = clientset.CoreV1().ConfigMaps("kube-system").Update(ctx, corednscustom, metav1.UpdateOptions{})
	if err != nil {
		fmt.Errorf("error: Updating ConfigMap: %s", err.Error())
		return
	}
}
