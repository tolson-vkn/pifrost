package watcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	fake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/tolson-vkn/pifrost/provider"
)

func TestPollService(t *testing.T) {
	// Test Case 1: Poll for an service object
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "example-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeLoadBalancer,
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(service)

	_, err := pollService(fakeClient, service)
	if err != nil {
		t.Errorf("Service poll error: %s", err)
	}
}

func TestAddServiceLB(t *testing.T) {
	mockServer, serverURL := startMockServer(t)
	defer mockServer.Close()

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
			Annotations: map[string]string{
				"pifrost.tolson.io/domain": "example.com",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "example-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeLoadBalancer,
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	mockPHR, err := provider.InitDNSProvider(true, serverURL, "mocktoken")
	if err != nil {
		t.Errorf("Service handler test error: %s", err)
	}

	fakeClient := fake.NewSimpleClientset(service)

	err = addServiceHandler(fakeClient, mockPHR, service)
	if err != nil {
		t.Errorf("Service handler test error: %s", err)
	}
}

func TestDelServiceLB(t *testing.T) {
	mockServer, serverURL := startMockServer(t)
	defer mockServer.Close()

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
			Annotations: map[string]string{
				"pifrost.tolson.io/domain": "example.com",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "example-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeLoadBalancer,
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	mockPHR, err := provider.InitDNSProvider(true, serverURL, "mocktoken")
	if err != nil {
		t.Errorf("Service handler test error: %s", err)
	}

	fakeClient := fake.NewSimpleClientset(service)

	err = delServiceHandler(fakeClient, mockPHR, service)
	if err != nil {
		t.Errorf("Service handler test error: %s", err)
	}
}

func TestUpdateServiceLB(t *testing.T) {
	// Test case 1: Update a service object
	// State tracker for the httptest server
	var added int = 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var mockResponse string
		action := r.URL.Query().Get("action")
		switch action {
		case "get":
			if added > 0 {
				mockResponse = `{"data":[["example.com","192.168.1.2"]]}[]`
			} else if added > 1 {
				mockResponse = `{"data":[["new.example.com","192.168.1.2"],["example.com","192.168.1.2"]]}[]`
			} else if added > 2 {
				mockResponse = `{"data":[["new.example.com","192.168.1.2"]]}[]`
			}
			added = added + 1
		case "add":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		case "delete":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer mockServer.Close()

	mockPHR, err := provider.InitDNSProvider(true, strings.Replace(mockServer.URL, "http://", "", 1), "mocktoken")
	if err != nil {
		t.Errorf("Service handler test error: %s", err)
	}

	oldService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
			Annotations: map[string]string{
				"pifrost.tolson.io/domain": "example.com",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "example-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeLoadBalancer,
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcherStarted := make(chan struct{})
	client := fake.NewSimpleClientset(oldService)
	client.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := client.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, watch, nil
	})

	services := make(chan *v1.Service, 1)
	informers := informers.NewSharedInformerFactory(client, 0)
	serviceInformer := informers.Core().V1().Services().Informer()

	serviceInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObject, newObject interface{}) {
			oS := oldObject.(*v1.Service)
			nS := newObject.(*v1.Service)

			updateServiceHandler(client, mockPHR, oS, nS)
			services <- nS
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), serviceInformer.HasSynced)
	<-watcherStarted

	newService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-service",
			Namespace: "default",
			Annotations: map[string]string{
				"pifrost.tolson.io/domain": "new.example.com",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "example-app",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeLoadBalancer,
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	_, err = client.CoreV1().Services(newService.ObjectMeta.Namespace).Update(context.TODO(), newService, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("error injecting pod add: %v", err)
	}

	select {
	case service := <-services:
		t.Logf("Got service from channel: %s/%s", service.Namespace, service.Name)
	case <-time.After(wait.ForeverTestTimeout):
		t.Error("Informer did not get the added service")
	}
}
