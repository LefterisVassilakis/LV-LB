package main

import (
	"bufio"
	"fmt"
	"strings"

	"lv-lb.com/controller"

	"context"
	"log"
	"os"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)


func main() {
	if len(os.Args) != 2 {
        log.Println("Usage: go run main.go [internal_state]")
        os.Exit(1)
    }

	internal_state := os.Args[1]
	internal_state_bool, err := strconv.ParseBool(internal_state)
    if err != nil {
        log.Fatal(err)
    }

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter the IP of your router: ")
	router_ip, _ := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	router_ip = strings.TrimSuffix(router_ip, "\n")
	router_ip = "https://" + router_ip

	fmt.Printf("Enter the username for your router: ")
	router_username, _ := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	router_username = strings.TrimSuffix(router_username, "\n")

	fmt.Printf("Enter the password for your router: ")
	router_password, _ := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	router_password = strings.TrimSuffix(router_password, "\n")

	fmt.Printf("Enter the IP of your node: ")
	node_ip, _ := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	node_ip = strings.TrimSuffix(node_ip, "\n")

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

	lb_controller := controller.New(clientset, ctx, internal_state_bool, node_ip)
	// fmt.Println(router_ip)
	// fmt.Println(router_username)
	// fmt.Println(router_password)
	lb_controller.ConnectClient(router_ip, router_username, router_password)
	lb_controller.Controller_loop()
}
