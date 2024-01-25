package watcher

import (
	"fmt"
	"reflect"
	"sort"

	v1 "k8s.io/api/core/v1"
	v1Networking "k8s.io/api/networking/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func getSvcAnnotation(annotations map[string]string) (string, bool) {
	if val, ok := annotations["pifrost.tolson.io/domain"]; ok {
		return val, true
	} else {
		return "", false
	}
}

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

func convertToIngress(obj interface{}) (*v1Networking.Ingress, error) {
	dest, ok := obj.(*v1Networking.Ingress)
	if !ok {
		return nil, fmt.Errorf("cast failed %T to %T", obj, dest)
	}
	return dest, nil
}

func convertToService(obj interface{}) (*v1.Service, error) {
	dest, ok := obj.(*v1.Service)
	if !ok {
		return nil, fmt.Errorf("cast failed %T to %T", obj, dest)
	}
	return dest, nil
}

func sameHosts(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	copy1 := make([]string, len(s1))
	copy2 := make([]string, len(s2))
	copy(copy1, s1)
	copy(copy2, s2)

	sort.Strings(copy1)
	sort.Strings(copy2)

	return reflect.DeepEqual(copy1, copy2)
}

func hostsAddedRemovedBoth(s1, s2 []string) ([]string, []string, []string) {
	var removed, added, both []string
	countS1 := make(map[string]int)
	countS2 := make(map[string]int)

	for _, value := range s1 {
		countS1[value]++
	}

	for _, value := range s2 {
		countS2[value]++
	}

	// in s2 but not s1
	for key, count := range countS2 {
		if count > countS1[key] {
			added = append(added, key)
		}
	}

	// in s1 but not s2
	for key, count := range countS1 {
		if count > countS2[key] {
			removed = append(removed, key)
		}
	}

	// in both
	for key, count := range countS1 {
		if count > 0 && countS2[key] > 0 {
			both = append(both, key)
		}
	}

	return added, removed, both
}
