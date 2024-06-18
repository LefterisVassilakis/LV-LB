package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"testing"
	"time"
	"strconv"

	"lv-lb.com/controller"
)

var sleep_time = 2500 * time.Millisecond

var lb_controller *controller.Controller

var service = &corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "lb-test",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			"app": "my-app",
		},
		Ports: []corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
		},
		Type: corev1.ServiceTypeLoadBalancer,
	},
}

func setup() {
	// Get the path to the kubeconfig file.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	// Build kubeconfig from the specified path.
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create a new Kubernetes clientset.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	ctx := context.Background()

	lb_controller = controller.New(clientset, ctx, false)
	// lb_controller.ConnectClient("https://139.91.92.131", "ubnt", "raspberryk8s")

	lb_controller.ConnectClient("https://139.91.92.131", "ubnt", "raspberryk8s")
	go lb_controller.Controller_loop()
}

func TestMain(m *testing.M) {
	// Perform setup
	setup()

	// Run tests
	fmt.Println("Running tests...")
	code := m.Run()

	os.Exit(code)
}

func TestAddRule(t *testing.T) {

	// Create the service in the default namespace
	svc, err := lb_controller.Clientset.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating service: %v", err)
	}

	time.Sleep(sleep_time)

	for i := 0; i < 5; i++ {
		if lb_controller.LBsvc_has_FWrule(*svc) {
			break
		}
		time.Sleep(sleep_time)
	}
	if !lb_controller.LBsvc_has_FWrule(*svc) {
		t.Errorf("Service %s was not added to the firewall rules", svc.Name)
	}

	defer func() {
		lb_controller.Clientset.CoreV1().Services("default").Delete(context.TODO(), "lb-test", metav1.DeleteOptions{})
		FWrules := []map[string]string{}
		portForwards := make(map[string]string)
		portForwards["description"] = "LV-LB-" + svc.Name + "-" + strconv.Itoa(int(svc.Spec.Ports[0].NodePort))
		portForwards["forward-to-address"] = "192.168.1.108"
		portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[0].NodePort))
		portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[0].NodePort))
		portForwards["protocol"] = "tcp"

		FWrules = append(FWrules, portForwards)
		//log.Println("Rules to delete:", FWrules)
		lb_controller.Delete_FWrules(FWrules)
	}()
}

// func TestDeleteRule(t *testing.T) {

// 	// Create the service in the default namespace
// 	svc, err := lb_controller.Clientset.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
// 	if err != nil {
// 		log.Fatalf("Error creating service: %v", err)
// 	}

// 	// Wait for the service to be added to the firewall rules
//     for !lb_controller.LBsvc_has_FWrule(*svc){
// 		//time.Sleep(sleep_time)
// 	}

// 	err = lb_controller.Clientset.CoreV1().Services("default").Delete(context.TODO(), "lb-test", metav1.DeleteOptions{})
// 	if err != nil {
// 		log.Fatalf("Error deleting service: %v", err)
// 	}

// 	for i := 0; i < 5; i++ {
// 		if !lb_controller.LBsvc_has_FWrule(*svc) {
// 			break
// 		}
// 		time.Sleep(sleep_time)
// 	}
// 	if lb_controller.LBsvc_has_FWrule(*svc) {
// 		t.Errorf("Service %s was not added to the firewall rules", svc.Name)
// 	}

// }
