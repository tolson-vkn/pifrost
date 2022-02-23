package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	v1Networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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
	// These ought to be an interface, dones of duplicate code in here...
	go watcherIngress(dnsProvider, client, ingressAnnotation, ingressEIP, w)
	go watcherService(dnsProvider, client, w)

	w.Wait()
}

func watcherIngress(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, ingressAnnotation bool, ingressEIP string, w *sync.WaitGroup) {
	defer w.Done()

	logrus.Info("Starting ingress watcher...")
	if !ingressAnnotation {
		logrus.Info("Will only externalize dns for ingress with annotations.")
	} else {
		logrus.Info("Externalizing all ingress objects")
	}

	if len(ingressEIP) != 0 {
		logrus.Infof("Externalized ingress hosts will use IP: %s", ingressEIP)
	}

	watcher, err := client.NetworkingV1().Ingresses(v1.NamespaceAll).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to create service watcher client: %s", err)
	}

	for event := range watcher.ResultChan() {
		ingress := event.Object.(*v1Networking.Ingress)

		switch event.Type {
		case watch.Added:
			if !ingressAnnotation {
				ok := hasIngressAnnotation(ingress.Annotations)
				if !ok {
					continue
				}
			}

			var ip string

			// Use custom external IP, for installs that report ingress address node.
			if len(ingressEIP) == 0 {
				ingress, err = pollIngress(ingress, client)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Warnf("Watch error: %s", err)
					continue
				}

				ip = ingress.Status.LoadBalancer.Ingress[0].IP
				if len(ip) == 0 {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Warn("Ingress does not have LoadBalancerIP")
					continue
				}
			} else {
				ip = ingressEIP
			}
			for _, rule := range ingress.Spec.Rules {
				host := rule.Host
				logrus.WithFields(logrus.Fields{
					"ingress": ingress.ObjectMeta.Name,
					"domain":  host,
				}).Info("Adding ingress domain")

				changeSet, err := provider.CreateChangeSet(ip, host, "add")
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
						"domain":  host,
					}).Fatalf("Could not create add changeset: %s", err)
				}

				err = dnsProvider.ModifyDNS(changeSet)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
						"domain":  host,
					}).Fatalf("Could not create record: %s", err)
				}

				logrus.WithFields(logrus.Fields{
					"ingress": ingress.ObjectMeta.Name,
					"domain":  host,
				}).Info("Completed ingress domain")
			}
		case watch.Deleted:
			if !ingressAnnotation {
				ok := hasIngressAnnotation(ingress.Annotations)
				if !ok {
					continue
				}
			}

			var ip string

			// Use custom external IP, for installs that report ingress address node.
			if len(ingressEIP) == 0 {
				ingress, err = pollIngress(ingress, client)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Warnf("Watch error: %s", err)
					continue
				}

				ip = ingress.Status.LoadBalancer.Ingress[0].IP
				if len(ip) == 0 {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
					}).Warn("Ingress does not have LoadBalancerIP")
					continue
				}
			} else {
				ip = ingressEIP
			}
			for _, rule := range ingress.Spec.Rules {
				host := rule.Host
				logrus.WithFields(logrus.Fields{
					"ingress": ingress.ObjectMeta.Name,
					"domain":  host,
				}).Info("Deleting ingress domain")

				changeSet, err := provider.CreateChangeSet(ip, host, "delete")
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
						"domain":  host,
					}).Fatalf("Could not create add changeset: %s", err)
				}

				err = dnsProvider.ModifyDNS(changeSet)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"ingress": ingress.ObjectMeta.Name,
						"domain":  host,
					}).Fatalf("Could not create record: %s", err)
				}

				logrus.WithFields(logrus.Fields{
					"ingress": ingress.ObjectMeta.Name,
					"domain":  host,
				}).Info("Deleted ingress domain")
			}
		}
	}
}

func pollIngress(ingress *v1Networking.Ingress, client *kubernetes.Clientset) (*v1Networking.Ingress, error) {
	var count int = 1
	const tries int = 8
	for {
		ingress, err := client.NetworkingV1().Ingresses(ingress.ObjectMeta.Namespace).Get(context.TODO(), ingress.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to create ingress poll client: %s", err)
		}
		time.Sleep(500)
		if len(ingress.Status.LoadBalancer.Ingress) != 0 {
			return ingress, nil
		} else {
			time.Sleep(1 << count * time.Second)
		}
		count++
		if count == tries {
			return nil, fmt.Errorf("Service did not get ingress IP in time")
		}
	}
}

func watcherService(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, w *sync.WaitGroup) {
	defer w.Done()

	logrus.Info("Starting service watcher...")

	watcher, err := client.CoreV1().Services(v1.NamespaceAll).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Fatalf("Failed to create service watcher client: %s", err)
	}

	for event := range watcher.ResultChan() {
		svc := event.Object.(*v1.Service)

		switch event.Type {
		case watch.Added:
			val, ok := hasAnnotation(svc.Annotations)
			if ok {
				svc, err = pollService(svc, client)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
					}).Warnf("Watch error: %s", err)
					continue
				}
				if svc.Spec.Type == "LoadBalancer" {
					ip := svc.Status.LoadBalancer.Ingress[0].IP
					if len(ip) == 0 {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
						}).Warn("Has annotation but does not have LoadBalancerIP")
						continue
					}

					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
						"domain":  val,
					}).Info("Adding service domain with annotation")

					changeSet, err := provider.CreateChangeSet(ip, val, "add")
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
							"domain":  val,
						}).Fatalf("Could not create add changeset: %s", err)
					}

					err = dnsProvider.ModifyDNS(changeSet)
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
							"domain":  val,
						}).Fatalf("Could not create record: %s", err)
					}

					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
						"domain":  val,
					}).Info("Completed service domain")
				}
			}
		case watch.Deleted:
			val, ok := hasAnnotation(svc.Annotations)
			if ok {
				svc, err = pollService(svc, client)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
					}).Warnf("Watch error: %s", err)
					continue
				}
				if svc.Spec.Type == "LoadBalancer" {
					ip := svc.Status.LoadBalancer.Ingress[0].IP
					if len(ip) == 0 {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
						}).Warn("Has annotation but does not have LoadBalancerIP")
						continue
					}

					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
						"domain":  val,
					}).Info("Deleting service domain with annotation")

					changeSet, err := provider.CreateChangeSet(ip, val, "delete")
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
							"domain":  val,
						}).Fatalf("Could not create delete changeset: %s", err)
					}

					err = dnsProvider.ModifyDNS(changeSet)
					if err != nil {
						logrus.WithFields(logrus.Fields{
							"service": svc.ObjectMeta.Name,
							"domain":  val,
						}).Fatalf("Could not delete record: %s", err)
					}

					logrus.WithFields(logrus.Fields{
						"service": svc.ObjectMeta.Name,
						"domain":  val,
					}).Info("Deleted service domain with annotation")
				}
			}
		}
	}
}

func pollService(svc *v1.Service, client *kubernetes.Clientset) (*v1.Service, error) {
	var count int = 1
	const tries int = 10
	for {
		svc, err := client.CoreV1().Services(svc.ObjectMeta.Namespace).Get(context.TODO(), svc.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to create service poll client: %s", err)
		}
		time.Sleep(500)
		if len(svc.Status.LoadBalancer.Ingress) != 0 {
			return svc, nil
		} else {
			time.Sleep(500)
		}
		count++
		if count == tries {
			return nil, fmt.Errorf("Service did not get external IP in time")
		}
	}
}

func hasAnnotation(annotations map[string]string) (string, bool) {
	if val, ok := annotations["pifrost.tolson.io/domain"]; ok {
		return val, true
	} else {
		return "", false
	}
}

// Bad...
func hasIngressAnnotation(annotations map[string]string) bool {
	if val, ok := annotations["pifrost.tolson.io/ingress"]; ok {
		if val == "true" {
			return true
		}
		return false
	} else {
		return false
	}
}
