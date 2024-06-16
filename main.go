package main

import (
	"lv-lb.com/controller"

	"context"
	"log"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)


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

	lb_controller := controller.New(clientset, ctx, true)
	lb_controller.ConnectClient("https://139.91.92.131", "ubnt", "raspberryk8s")
	lb_controller.Controller_loop()
}
