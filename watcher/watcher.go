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
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/tolson-vkn/pifrost/provider"
)

func Watch(dnsProvider *provider.PiHoleRequest, kconfig *rest.Config, ingressAnnotation bool, ingressEIP string) {
	client, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logrus.Fatal("Could not create kubeconfig")
	}
	w := &sync.WaitGroup{}

	w.Add(2)
	go watcherIngress(client, dnsProvider, ingressAnnotation, ingressEIP, w)
	go watcherService(client, dnsProvider, w)
	w.Wait()
}

func watcherIngress(client kubernetes.Interface, dnsProvider *provider.PiHoleRequest, ingressAnnotation bool, ingressEIP string, w *sync.WaitGroup) {
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

				err = addIngressHandler(
					client, dnsProvider, ingressAnnotation, ingressEIP, ingress,
				)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				ingress, err := convertToIngress(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				err = delIngressHandler(
					client, dnsProvider, ingressAnnotation, ingressEIP, ingress,
				)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
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

				err = updateIngressHandler(
					client, dnsProvider, ingressAnnotation, ingressEIP,
					oldIngress, newIngress,
				)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": oldIngress.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
			},
		},
	)

	go controller.Run(wait.NeverStop)
	for {
		time.Sleep(time.Second)
	}
}

func watcherService(client kubernetes.Interface, dnsProvider *provider.PiHoleRequest, w *sync.WaitGroup) {
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
					logrus.WithFields(logrus.Fields{
						"service": service.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}

				err = addServiceHandler(client, dnsProvider, service)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"service": service.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				service, err := convertToService(obj)
				if err != nil {
					logrus.Fatalf("Watch error: %s", err)
				}

				err = delServiceHandler(client, dnsProvider, service)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"service": service.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
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

				err = updateServiceHandler(client, dnsProvider, oldService, newService)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"service": oldService.ObjectMeta.Name,
					}).Fatalf("Watch error: %s", err)
				}
			},
		},
	)

	go controller.Run(wait.NeverStop)
	for {
		time.Sleep(time.Second)
	}
}
