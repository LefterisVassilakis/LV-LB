package main

import (
	"lv-lb.com/controller"

	"context"
	"log"
	"os"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)


func main() {
	// Check for correct number of arguments
	if len(os.Args) != 6 {
        log.Println("Usage: go run main.go [state router_ip router_username router_password node_ip]")
        os.Exit(1)
    }

	// Get the state of the load balancer
	state := os.Args[1]
	state_bool, err := strconv.ParseBool(state)
    if err != nil {
        log.Fatal(err)
    }

	// Get the router ip, username, and password and the node ip
	router_ip := os.Args[2]
	router_ip = "https://" + router_ip
	router_username := os.Args[3]
	router_password := os.Args[4]
	node_ip := os.Args[5]


	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	ctx := context.Background()

	lb_controller := controller.New(clientset, ctx, state_bool, node_ip)
	lb_controller.ConnectClient(router_ip, router_username, router_password)
	lb_controller.Controller_loop()
}







