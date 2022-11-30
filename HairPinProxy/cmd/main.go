package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"k8s.io/client-go/kubernetes"
)

var (
	kubeconfig *string
)

//config, err := rest.InClusterConfig()
//if err != nil {
/*	fmt.Errorf("error: Getting In CLuster Config: %s", err.Error())
	os.Exit(1)
}*/

type Reconciler struct {
	closed chan struct{}
	ticker *time.Ticker
}

func (r *Reconciler) Run(ctx context.Context, clienset *kubernetes.Clientset) {
	for {
		select {
		case <-r.closed:
			return
		case <-r.ticker.C:
			reconcile(ctx, clienset)
		}
	}
}

func main() {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, "Downloads", "kubeconfig--garden--mon.yaml"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	defer func() {
		signal.Stop(c)
	}()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(fmt.Errorf("error: creating ClientSet: %s", err.Error()))
		os.Exit(1)
	}

	reconciler := &Reconciler{
		closed: make(chan struct{}),
		ticker: time.NewTicker(time.Second * 15),
	}

	go func() {
		select {
		case sig := <-c:
			fmt.Printf("Got %s signal. Aborting...\n", sig)
			cancel()
			close(reconciler.closed)
		}
	}()

	reconciler.Run(ctx, clientset)
}

func reconcile(ctx context.Context, clientset *kubernetes.Clientset) {
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Println(fmt.Errorf("error: Getting Ingresses Error: %s", err.Error()))
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
		Hosts = append(Hosts, TLSHost.Hosts...)
	}
	for _, Host := range Hosts {
		dnsConfig.WriteString(fmt.Sprintf("rewrite name %s hairpin-proxy.hairpin-proxy.svc.cluster.local\n", Host))
	}
	corednscustom, err := clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns-custom", metav1.GetOptions{})
	if err != nil {
		fmt.Println(fmt.Errorf("error: Getting ConfigMap: %s", err.Error()))
		return
	}
	fmt.Println(dnsConfig.String())

	corednscustom.Data["changeme.override"] = dnsConfig.String()

	bla := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test123",
			Namespace: "kube-system",
		},
		Data: make(map[string]string),
	}
	bla.Data["changeme.override"] = dnsConfig.String()

	_, err = clientset.CoreV1().ConfigMaps("kube-system").Create(ctx, &bla, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(fmt.Errorf("error: Updating ConfigMap: %s", err.Error()))
		return
	}
}
