package controller

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"astuart.co/edgeos-rest/pkg/edgeos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct{
	Clientset *kubernetes.Clientset
	Context context.Context
	LBservices []corev1.Service
	FWrules []edgeos.PortForward
	EdgeClient *edgeos.Client
}

func New(clientset *kubernetes.Clientset, ctx context.Context) *Controller {
	return &Controller{
		Clientset: clientset,
		Context: ctx,
		LBservices: []corev1.Service{},
		FWrules: []edgeos.PortForward{},
		EdgeClient: nil,
	}
}

func (c *Controller) ConnectClient(addr string, username string, password string) {
	client, err := edgeos.NewClient(addr, username, password)
	if err != nil {
		log.Fatal(err)
	}

	if err := client.Login(); err != nil {
		log.Fatal(err)
	}
	
	c.EdgeClient = client
}

func (c *Controller) listSVC() *corev1.ServiceList {
	services, err := c.Clientset.CoreV1().Services("").List(c.Context, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing services: %v", err)
	}
	return services
}

func (c *Controller) listFW() *edgeos.PortForwards {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	data := feat["data"].(map[string]interface{})["rules-config"].([]interface{})
	fmt.Println(reflect.TypeOf(data))
	return data
}

func contains(slice interface{}, item interface{}) bool {
	valueof := reflect.ValueOf(slice)
	if valueof.Kind() != reflect.Slice {
		panic("contains() expects a slice")
	}

	for i := 0; i < valueof.Len(); i++ {
		if reflect.DeepEqual(valueof.Index(i).Interface(), item) {
            return true
        }
	}
	return false
}

func (c *Controller) listLBSVC() {
	for _, svc := range c.LBservices {
		fmt.Println(svc.Name, svc.Spec.Ports[0].Port, svc.Spec.Ports[0].NodePort)
	}
	fmt.Println()
}

func (c *Controller) listFWrules() {
	for _, rule := range c.FWrules {
		fmt.Println(rule)
	}
}

func (c *Controller) Reconcile() {
	services := c.listSVC()
	for _, svc := range services.Items {
		// log.Println(svc)
		// log.Printf("%s    %s    %s", svc.Name, svc.Namespace, svc.Spec.Type)
		if svc.Spec.Type == "LoadBalancer" && !contains(c.LBservices, svc){
			log.Printf("Adding %s to LBservices", svc.Name)
			c.LBservices = append(c.LBservices, svc)
			c.listLBSVC()

			// Set external IP of service as the edge router IP

			// Add port forwarding rule if not already present
		}
	}
}

func (c *Controller) Controller_loop(){
	for {
		c.Reconcile()
		time.Sleep(2 * time.Second)
	}
}