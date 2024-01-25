package watcher

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	v1Networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"github.com/tolson-vkn/pifrost/provider"
)

func Watch(dnsProvider *provider.PiHoleRequest, kconfig *rest.Config, ingressAnnotation bool, ingressEIP string) {
	client, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logrus.Fatal("Could not create kubeconfig")
	}
	w := &sync.WaitGroup{}

	w.Add(2)
	go watcherIngress(dnsProvider, client, ingressAnnotation, ingressEIP, w)
	go watcherService(dnsProvider, client, w)
	w.Wait()
}

func watcherIngress(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, ingressAnnotation bool, ingressEIP string, w *sync.WaitGroup) {
	logrus.Info("Starting ingress watcher...")
	if !ingressAnnotation {
		logrus.Info("Will only externalize dns for ingress with annotations.")
	} else {
		logrus.Info("Externalizing all ingress objects")
	}

	if len(ingressEIP) != 0 {
		logrus.Infof("Externalized ingress hosts will use IP: %s", ingressEIP)
	}

	watchlist := cache.NewListWatchFromClient(
		client.NetworkingV1().RESTClient(),
		"ingresses",
		v1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&v1Networking.Ingress{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ingress, err := convertToIngress(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				addIngressHandler(
					ingressAnnotation, ingressEIP, client, dnsProvider, ingress,
				)
			},
			DeleteFunc: func(obj interface{}) {
				ingress, err := convertToIngress(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				delIngressHandler(
					ingressAnnotation, ingressEIP, client, dnsProvider, ingress,
				)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldIngress, err := convertToIngress(oldObj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				newIngress, err := convertToIngress(newObj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				updateIngressHandler(
					ingressAnnotation, ingressEIP, client,
					dnsProvider, oldIngress, newIngress,
				)
			},
		},
	)

	go controller.Run(wait.NeverStop)
	for {
		time.Sleep(time.Second)
	}
}

func watcherService(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, w *sync.WaitGroup) {
	logrus.Info("Starting service watcher...")

	watchlist := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"services",
		v1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Service{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service, err := convertToService(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				addServiceHandler(dnsProvider, client, service)
			},
			DeleteFunc: func(obj interface{}) {
				service, err := convertToService(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				delServiceHandler(dnsProvider, client, service)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldService, err := convertToService(oldObj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				newService, err := convertToService(newObj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				updateServiceHandler(dnsProvider, client, oldService, newService)
			},
		},
	)

	go controller.Run(wait.NeverStop)
	for {
		time.Sleep(time.Second)
	}
}
