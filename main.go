package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	rtCfgClientset "github.com/devopsext/configurator/pkg/generated/clientset/versioned"
	informers "github.com/devopsext/configurator/pkg/generated/informers/externalversions"
	"github.com/devopsext/configurator/pkg/signals"
)

var (
	kubeconfig string
	config string
	selfNamespace string
	selfPodName string
)

//TODO: Add metrics in prometheus format
func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if config == "" || selfNamespace=="" || selfPodName==""  {
		flag.Usage()
		os.Exit(1)
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()


	if kubeconfig == "" { //try to source kubeconfig from std. env. var KUBECONFIG
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes kubeClient: %s", err.Error())
	}

	rtCfgClient, err := rtCfgClientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example rtCfgClient: %s", err.Error())
	}

	rtCfgInformerFactory := informers.NewSharedInformerFactory(rtCfgClient, time.Second*30)

	controller,err := NewController(kubeClient, rtCfgClient,
		rtCfgInformerFactory.Rtcfg().V1alpha1().Generics(),config,selfNamespace,selfPodName)

	if err!=nil {
		panic(err)
	}

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	rtCfgInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. If not specified, then env. var KUBECONFIG examined. Only required if out-of-cluster.")
	flag.StringVar(&config, "config", "", "Path to controller config file.")
	flag.StringVar(&selfNamespace, "namespace", "", "k8s namespace where this application runs.")
	flag.StringVar(&selfPodName, "podname", "", "k8s pod name where this application runs.")
}
