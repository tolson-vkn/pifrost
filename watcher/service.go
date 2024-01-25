package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/tolson-vkn/pifrost/provider"
)

func pollService(svc *v1.Service, client *kubernetes.Clientset) (*v1.Service, error) {
	var count int = 1
	const tries int = 10
	for {
		svc, err := client.CoreV1().Services(svc.ObjectMeta.Namespace).Get(context.TODO(), svc.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to create service poll client: %s", err)
		}
		if len(svc.Status.LoadBalancer.Ingress) != 0 {
			return svc, nil
		} else {
			time.Sleep(1 << count * time.Second)
		}
		count++
		if count == tries {
			return nil, fmt.Errorf("Service did not get external IP in time")
		}
	}
}

func addServiceRecord(dnsProvider *provider.PiHoleRequest, host string, ip string) error {
	changeSet, err := provider.CreateChangeSet(ip, host, "add")
	if err != nil {
		return fmt.Errorf("Could not create add changeset: %s", err)
	}

	err = dnsProvider.ModifyDNS(changeSet)
	if err != nil {
		return fmt.Errorf("Could not create record: %s", err)
	}

	return nil
}

func delServiceRecord(dnsProvider *provider.PiHoleRequest, host string, ip string) error {
	changeSet, err := provider.CreateChangeSet(ip, host, "delete")
	if err != nil {
		return fmt.Errorf("Could not create delete changeset: %s", err)
	}

	err = dnsProvider.ModifyDNS(changeSet)
	if err != nil {
		return fmt.Errorf("Could not delete record: %s", err)
	}

	return nil
}

func addServiceHandler(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, service *v1.Service) {
	host, hasIt := getSvcAnnotation(service.Annotations)
	if hasIt {
		service, err := pollService(service, client)
		if err != nil {
			logrus.Fatalf("Watch error: %s", err)
		}

		if service.Spec.Type == "LoadBalancer" {
			ip := service.Status.LoadBalancer.Ingress[0].IP
			if len(ip) == 0 {
				logrus.WithFields(logrus.Fields{
					"service": service.ObjectMeta.Name,
				}).Warn("Has annotation but does not have LoadBalancerIP")
			}

			logrus.WithFields(logrus.Fields{
				"service": service.ObjectMeta.Name,
				"domain":  host,
			}).Info("Adding service domain with annotation")

			err = addServiceRecord(dnsProvider, host, ip)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"service": service.ObjectMeta.Name,
					"domain":  host,
				}).Fatalf("Record creation error: %s", err)
			}

			logrus.WithFields(logrus.Fields{
				"service": service.ObjectMeta.Name,
				"domain":  host,
			}).Info("Completed service creation for domain")
		} else {
			logrus.WithFields(logrus.Fields{
				"service": service.ObjectMeta.Name,
			}).Warn("Service is not of type LoadBalancer. Ignored")
		}
	}
}

func delServiceHandler(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, service *v1.Service) {
	host, hasIt := getSvcAnnotation(service.Annotations)
	if hasIt {
		if service.Spec.Type == "LoadBalancer" {
			ip := service.Status.LoadBalancer.Ingress[0].IP
			if len(ip) == 0 {
				logrus.WithFields(logrus.Fields{
					"service": service.ObjectMeta.Name,
				}).Fatalf("Has annotation but does not have LoadBalancerIP")
			}

			logrus.WithFields(logrus.Fields{
				"service": service.ObjectMeta.Name,
				"domain":  host,
			}).Info("Deleting service domain with annotation")

			err := delServiceRecord(dnsProvider, host, ip)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"service": service.ObjectMeta.Name,
					"domain":  host,
				}).Fatalf("Record deletion error: %s", err)
			}

			logrus.WithFields(logrus.Fields{
				"service": service.ObjectMeta.Name,
				"domain":  host,
			}).Info("Completed service deletion for domain")
		}
	}
}

func updateServiceHandler(dnsProvider *provider.PiHoleRequest, client *kubernetes.Clientset, oldService *v1.Service, newService *v1.Service) {
	oldHost, oldHasIt := getSvcAnnotation(oldService.Annotations)
	newHost, newHasIt := getSvcAnnotation(newService.Annotations)

	// LB type changed.
	if newService.Spec.Type != "LoadBalancer" {
		logrus.WithFields(logrus.Fields{
			"service": newService.ObjectMeta.Name,
		}).Warn("Service is not of type LoadBalancer. Ignored")
		return
	}

	// Condition where pending IP is now assigned is captured by add event...
	if oldHost == newHost && len(oldService.Status.LoadBalancer.Ingress) == 0 &&
		len(newService.Status.LoadBalancer.Ingress) > 0 {

		logrus.WithFields(logrus.Fields{
			"service": newService.ObjectMeta.Name,
		}).Debug("LB IP skip condition")

		return
	}

	if len(oldService.Status.LoadBalancer.Ingress) != 1 || len(newService.Status.LoadBalancer.Ingress) != 1 {
		logrus.WithFields(logrus.Fields{
			"service": newService.ObjectMeta.Name,
		}).Fatalf("pifrost only supports single LB IP service objects both service objects have LB IP issues")
	}

	oldIP := oldService.Status.LoadBalancer.Ingress[0].IP
	if len(oldIP) == 0 {
		logrus.WithFields(logrus.Fields{
			"service": oldService.ObjectMeta.Name,
		}).Fatalf("Has annotation but does not have LoadBalancerIP")
	}
	newIP := newService.Status.LoadBalancer.Ingress[0].IP
	if len(newIP) == 0 {
		logrus.WithFields(logrus.Fields{
			"service": newService.ObjectMeta.Name,
		}).Fatalf("Has annotation but does not have LoadBalancerIP")
	}

	// Was unmanaged. Now wants to manage.
	if !oldHasIt && newHasIt {
		err := addServiceRecord(dnsProvider, newHost, newIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"service": newService.ObjectMeta.Name,
				"domain":  newHost,
			}).Fatalf("Record creation error: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"service": newService.ObjectMeta.Name,
			"domain":  newHost,
		}).Info("Completed service management for domain")
	}

	// Was managed. Now wish to unmanage.
	if oldHasIt && !newHasIt {
		// Removing old host becuase that has the registered record.
		err := delServiceRecord(dnsProvider, oldHost, oldIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"service": oldService.ObjectMeta.Name,
				"domain":  oldHost,
			}).Fatalf("Record deletion error: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"service": oldService.ObjectMeta.Name,
			"domain":  oldHost,
		}).Info("No longer managing record. Removed")
	}

	// It was always managed, but something else changed...
	if oldHasIt && newHasIt {
		if oldHost != newHost || oldIP != newIP {
			err := delServiceRecord(dnsProvider, oldHost, oldIP)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"service": oldService.ObjectMeta.Name,
					"domain":  oldHost,
				}).Fatalf("Record deletion error: %s", err)
			}

			err = addServiceRecord(dnsProvider, newHost, newIP)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"service": newService.ObjectMeta.Name,
					"domain":  newHost,
				}).Fatalf("Record creation error: %s", err)
			}

			logrus.WithFields(logrus.Fields{
				"service": oldService.ObjectMeta.Name,
				"domain":  oldHost,
			}).Info("Record updated")
		}
	}

	// If any of the conditions above didn't evaluate... I don't care about it.
	return
}
