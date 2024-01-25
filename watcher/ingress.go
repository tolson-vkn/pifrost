package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	v1Networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/tolson-vkn/pifrost/provider"
)

func pollIngress(ingress *v1Networking.Ingress, client *kubernetes.Clientset) (*v1Networking.Ingress, error) {
	var count int = 1
	const tries int = 8
	for {
		ingress, err := client.NetworkingV1().Ingresses(ingress.ObjectMeta.Namespace).Get(context.TODO(), ingress.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to create ingress poll client: %s", err)
		}
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

func fetchIngressLB(client *kubernetes.Clientset, ingress *v1Networking.Ingress) (string, error) {
	// Ingress ExternalIP is not set... look it up.
	ingress, err := pollIngress(ingress, client)
	if err != nil {
		return "", fmt.Errorf("Watch error: %s", err)
	}

	if len(ingress.Status.LoadBalancer.Ingress) != 1 {
		return "", fmt.Errorf("pifrost only supports single LB IP ingress objects")
	}

	ip := ingress.Status.LoadBalancer.Ingress[0].IP
	if len(ip) == 0 {
		return "", fmt.Errorf("Ingress does not have LoadBalancerIP")
	}

	return ip, nil
}

func addIngressRecord(dnsProvider *provider.PiHoleRequest, host string, ip string) error {
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

func delIngressRecord(dnsProvider *provider.PiHoleRequest, host string, ip string) error {
	changeSet, err := provider.CreateChangeSet(ip, host, "delete")
	if err != nil {
		return fmt.Errorf("Could not create delete changeset: %s", err)
	}

	err = dnsProvider.ModifyDNS(changeSet)
	if err != nil {
		return fmt.Errorf("Could not create record: %s", err)
	}

	return nil
}

func addIngressHandler(ingressAnnotation bool, ingressIP string, client *kubernetes.Clientset, dnsProvider *provider.PiHoleRequest, ingress *v1Networking.Ingress) {
	if !ingressAnnotation {
		ok := hasIngressAnnotation(ingress.Annotations)
		if !ok {
			return
		}
	}

	var err error
	if len(ingressIP) == 0 {
		ingressIP, err = fetchIngressLB(client, ingress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": ingress.ObjectMeta.Name,
			}).Fatalf("Ingress LB error: %s", err)
		}
	}

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host

		err = addIngressRecord(dnsProvider, host, ingressIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": ingress.ObjectMeta.Name,
				"domain":  host,
			}).Fatalf("Record creation error: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"ingress": ingress.ObjectMeta.Name,
			"domain":  host,
		}).Info("Completed ingress creation for domain")
	}
}

func delIngressHandler(ingressAnnotation bool, ingressIP string, client *kubernetes.Clientset, dnsProvider *provider.PiHoleRequest, ingress *v1Networking.Ingress) {
	if !ingressAnnotation {
		ok := hasIngressAnnotation(ingress.Annotations)
		if !ok {
			return
		}
	}

	var err error
	if len(ingressIP) == 0 {
		ingressIP, err = fetchIngressLB(client, ingress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": ingress.ObjectMeta.Name,
			}).Fatalf("Ingress LB error: %s", err)
		}
	}

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host

		err = delIngressRecord(dnsProvider, host, ingressIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": ingress.ObjectMeta.Name,
				"domain":  host,
			}).Fatalf("Record deletion error: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"ingress": ingress.ObjectMeta.Name,
			"domain":  host,
		}).Info("Completed ingress deletion for domain")
	}
}

func updateIngressHandler(ingressAnnotation bool, ingressIP string, client *kubernetes.Clientset, dnsProvider *provider.PiHoleRequest, oldIngress *v1Networking.Ingress, newIngress *v1Networking.Ingress) {
	if !ingressAnnotation {
		newHasAnnotation := hasIngressAnnotation(newIngress.Annotations)
		oldHasAnnotation := hasIngressAnnotation(oldIngress.Annotations)

		// We no longer wish to manage this record. Remove it from pihole.
		if oldHasAnnotation && !newHasAnnotation {
			for _, host := range oldIngress.Spec.Rules {
				delIngressRecord(dnsProvider, host.Host, ingressIP)
			}

			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
			}).Info("Ingress no longer managed by pifrost")
		}
	}

	var err error
	var sameIP bool = false

	if len(ingressIP) == 0 {
		ingressIP, err = fetchIngressLB(client, newIngress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
			}).Fatalf("Ingress LB error: %s", err)
		}
	}

	var oldHosts []string
	for _, host := range oldIngress.Spec.Rules {
		oldHosts = append(oldHosts, host.Host)
	}

	var newHosts []string
	for _, host := range newIngress.Spec.Rules {
		newHosts = append(newHosts, host.Host)
	}

	// 1. Are they the same hosts?
	// 2. Are they the same LB IP?
	// 3. There are new hosts... Create new hosts
	// 4. There are hosts removed from the old one. Remove old hosts.
	// 5. The LB IP changed? Report for all.
	sameIngressHosts := sameHosts(oldHosts, newHosts)

	if len(ingressIP) == 0 {
		if len(oldIngress.Status.LoadBalancer.Ingress) != 1 || len(newIngress.Status.LoadBalancer.Ingress) != 1 {
			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
			}).Fatal("pifrost only supports single LB IP ingress objects")
		}

		if oldIngress.Status.LoadBalancer.Ingress[0].IP != newIngress.Status.LoadBalancer.Ingress[0].IP {
			ingressIP = newIngress.Status.LoadBalancer.Ingress[0].IP
			sameIP = false
		}
	} else {
		sameIP = true
	}

	if sameIngressHosts && sameIP {
		logrus.WithFields(logrus.Fields{
			"ingress": oldIngress.ObjectMeta.Name,
		}).Debug("There was a object update but nothing to do")

		return
	}

	// Get new old and instances in both...
	added, removed, both := hostsAddedRemovedBoth(oldHosts, newHosts)

	// Add the new records which are added new object
	for _, host := range added {
		err := addIngressRecord(dnsProvider, host, ingressIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
			}).Fatalf("Could not update ingress: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"ingress": oldIngress.ObjectMeta.Name,
			"domain":  host,
		}).Info("Completed ingress creation for domain")
	}

	// Remove the records now not present in new but are in old
	for _, host := range removed {
		err := delIngressRecord(dnsProvider, host, ingressIP)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
			}).Fatalf("Could not update ingress: %s", err)
		}

		logrus.WithFields(logrus.Fields{
			"ingress": oldIngress.ObjectMeta.Name,
			"domain":  host,
		}).Info("Completed ingress deletion for domain")
	}

	// They are the same but the LB ip changed. Skipped if using externalIP flag
	// nothing to do with those they're unchanged
	if !sameIP {
		for _, host := range both {
			err := delIngressRecord(dnsProvider, host, ingressIP)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"ingress": oldIngress.ObjectMeta.Name,
				}).Fatalf("Could not update ingress: %s", err)
			}

			err = addIngressRecord(dnsProvider, host, ingressIP)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"ingress": oldIngress.ObjectMeta.Name,
				}).Fatalf("Could not update ingress: %s", err)
			}

			logrus.WithFields(logrus.Fields{
				"ingress": oldIngress.ObjectMeta.Name,
				"domain":  host,
			}).Info("Completed ingress creation for domain")
		}
	}

	return
}
