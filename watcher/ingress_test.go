package watcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1Networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	fake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/tolson-vkn/pifrost/provider"
)

func TestPollIngress(t *testing.T) {
	// Test Case 1: Poll for an ingress object
	ingress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
					{
						IP: "192.168.1.2",
					},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(ingress)

	_, err := pollIngress(fakeClient, ingress)
	if err != nil {
		t.Errorf("Ingress poll error: %s", err)
	}
}

func TestFetchIngressLB(t *testing.T) {
	// Test case 1: Good case
	ingress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(ingress)
	_, err := fetchIngressLB(fakeClient, ingress)
	if err != nil {
		t.Errorf("Ingress poll error: %s", err)
	}

	// Test case 2: More IPs than supported
	invalidFetch := []v1Networking.IngressLoadBalancerIngress{
		{
			IP: "192.168.5.1",
		},
		{
			IP: "192.168.5.2",
		},
	}

	ingress.Status.LoadBalancer.Ingress = invalidFetch
	fakeClient = fake.NewSimpleClientset(ingress)
	_, err = fetchIngressLB(fakeClient, ingress)
	if err.Error() != "pifrost only supports single LB IP ingress objects" {
		t.Error("Fetch should have errored")
	}

	// Test case 3: Doesn't yet have a LB from controller.
	hasNoIPFetch := []v1Networking.IngressLoadBalancerIngress{{}}

	ingress.Status.LoadBalancer.Ingress = hasNoIPFetch
	fakeClient = fake.NewSimpleClientset(ingress)
	_, err = fetchIngressLB(fakeClient, ingress)
	if err.Error() != "Ingress does not have a LoadBalancerIP" {
		t.Error("Fetch should have errored")
	}
}

func TestAddIngressLB(t *testing.T) {
	mockServer, serverURL := startMockServer(t)
	defer mockServer.Close()

	ingress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	mockPHR, err := provider.InitDNSProvider(true, serverURL, "mocktoken")
	if err != nil {
		t.Errorf("Ingress handler test error: %s", err)
	}

	fakeClient := fake.NewSimpleClientset(ingress)

	addIngressHandler(fakeClient, mockPHR, true, "192.168.1.2", ingress)
}

func TestDelIngressLB(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		var mockResponse string
		action := r.URL.Query().Get("action")
		switch action {
		case "get":
			mockResponse = `{"data":[["example.com","192.168.1.2"]]}[]`
		case "add":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		case "delete":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer mockServer.Close()

	ingress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	mockPHR, err := provider.InitDNSProvider(true, strings.Replace(mockServer.URL, "http://", "", 1), "mocktoken")
	if err != nil {
		t.Errorf("Ingress handler test error: %s", err)
	}

	fakeClient := fake.NewSimpleClientset(ingress)

	delIngressHandler(fakeClient, mockPHR, true, "192.168.1.2", ingress)
}

func TestUpdateIngressLB(t *testing.T) {
	// Test case 1: Update an ingress object
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
		t.Errorf("Ingress handler test error: %s", err)
	}

	oldIngress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
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
	client := fake.NewSimpleClientset(oldIngress)
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

	ingresses := make(chan *v1Networking.Ingress, 1)
	informers := informers.NewSharedInformerFactory(client, 0)
	ingressInformer := informers.Networking().V1().Ingresses().Informer()

	ingressInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObject, newObject interface{}) {
			oI := oldObject.(*v1Networking.Ingress)
			nI := newObject.(*v1Networking.Ingress)

			updateIngressHandler(client, mockPHR, true, "192.168.1.2", oI, nI)
			ingresses <- nI
		},
	})

	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), ingressInformer.HasSynced)
	<-watcherStarted

	newIngress := &v1Networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: v1Networking.IngressSpec{
			Rules: []v1Networking.IngressRule{
				{
					Host: "new.example.com",
					IngressRuleValue: v1Networking.IngressRuleValue{
						HTTP: &v1Networking.HTTPIngressRuleValue{
							Paths: []v1Networking.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1Networking.IngressBackend{
										Service: &v1Networking.IngressServiceBackend{
											Name: "example-service",
											Port: v1Networking.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1Networking.IngressStatus{
			LoadBalancer: v1Networking.IngressLoadBalancerStatus{
				Ingress: []v1Networking.IngressLoadBalancerIngress{
					{
						IP: "192.168.5.1",
					},
				},
			},
		},
	}

	_, err = client.NetworkingV1().Ingresses(newIngress.ObjectMeta.Namespace).Update(context.TODO(), newIngress, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("error injecting pod add: %v", err)
	}

	select {
	case ingress := <-ingresses:
		t.Logf("Got ingress from channel: %s/%s", ingress.Namespace, ingress.Name)
	case <-time.After(wait.ForeverTestTimeout):
		t.Error("Informer did not get the added ingress")
	}
}
