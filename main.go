package main

import (
	"lvlb/controller"

	"context"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	corev1 "k8s.io/api/core/v1"
)


func getSVC(ctx context.Context, clientset *kubernetes.Clientset, namespace string, name string) *corev1.Service {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Error getting service: %v", err)
	}
	return svc
}

func updateSVC(ctx context.Context, clientset *kubernetes.Clientset, svc *corev1.Service) *corev1.Service {
	svc, err := clientset.CoreV1().Services(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Error updating service: %v", err)
	}
	return svc
}

func main() {
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
	
	lb_controller := controller.New(clientset, ctx)
	lb_controller.Reconcile()
}
