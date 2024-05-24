package controller

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"astuart.co/edgeos-rest/pkg/edgeos"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	Clientset  *kubernetes.Clientset
	Context    context.Context
	LBservices []corev1.Service
	FWrules    []map[string]string
	EdgeClient *edgeos.Client
}

func New(clientset *kubernetes.Clientset, ctx context.Context) *Controller {
	return &Controller{
		Clientset:  clientset,
		Context:    ctx,
		LBservices: []corev1.Service{},
		FWrules:    []map[string]string{},
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

func (c *Controller) listFW() []interface{} {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	data := feat["data"].(map[string]interface{})["rules-config"].([]interface{})
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

func (c *Controller) LBsvc_contains(svc corev1.Service) bool {
	for _, s := range c.LBservices {
		for _, port1 := range s.Spec.Ports {
			for _, port2 := range svc.Spec.Ports {
				if port1.NodePort == port2.NodePort {
					return true
				}
			}
		}
	}
	return false
}
func LBsvc_contains(svc corev1.Service, LBservices *corev1.ServiceList) bool {
	for _, s := range LBservices.Items {
		if s.Spec.Type != "LoadBalancer" {
			continue
		}
		for _, port1 := range s.Spec.Ports {
			for _, port2 := range svc.Spec.Ports {
				if port1.NodePort == port2.NodePort {
					return true
				}
			}
		}
	}
	return false
} 


func (c *Controller) Reconcile() {
	// Add new LB services
	services := c.listSVC()
	for _, svc := range services.Items {
		if svc.Spec.Type == "LoadBalancer" && !c.LBsvc_contains(svc) {
			// Set external IP of service as the edge router IP
			newIngress := v1.LoadBalancerIngress{
				IP:       "139.91.92.131",
				Hostname: "",
			}
			svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, newIngress)

			_, err := c.Clientset.CoreV1().Services(svc.Namespace).UpdateStatus(context.TODO(), &svc, metav1.UpdateOptions{})
    		if err != nil {
        		panic(err.Error())
    		}

			log.Printf("Adding %s to LBservices", svc.Name)
			c.LBservices = append(c.LBservices, svc)
			c.listLBSVC()

			// Add port forwarding rule if not already present
			feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
			if err != nil {
				log.Fatal(err)
			}

			index := len(c.LBservices) - 1
			portForwards := make(map[string]string)
			portForwards["description"] = "LV-LB-" + strconv.Itoa(index)
			portForwards["forward-to-address"] = "192.168.1.108"
			portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[0].NodePort))
			portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[0].NodePort))
			portForwards["protocol"] = "tcp"

			d := feat["data"].(map[string]interface{})["rules-config"].([]interface{})

			d = append(d, portForwards)

			c.FWrules = append(c.FWrules, portForwards)

			feat["data"].(map[string]interface{})["rules-config"] = d

			// log.Println(c.EdgeClient.SetFeature(edgeos.PortForwarding, feat["data"]))
			c.EdgeClient.SetFeature(edgeos.PortForwarding, feat["data"])
		}
	}

	// Remove LB services
	for i, svc := range c.LBservices {
		if !LBsvc_contains(svc, services) {
			log.Printf("Removing %s from LBservices", svc.Name)
			c.LBservices = append(c.LBservices[:i], c.LBservices[i+1:]...)
			log.Printf("Remove FW rule for service")
			FWrules := c.listFW()
			for i, rule := range FWrules {
				if strings.Contains(rule.(map[string]interface{})["description"].(string), "LV-LB-") && 
				rule.(map[string]interface{})["forward-to-port"].(string) == strconv.Itoa(int(svc.Spec.Ports[0].NodePort)) && 
				rule.(map[string]interface{})["forward-to-address"].(string) == "192.168.1.108" {
					FWrules = append(FWrules[:i], FWrules[i+1:]...)
					feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
					if err != nil {
						log.Fatal(err)
					}
					feat["data"].(map[string]interface{})["rules-config"] = FWrules
					c.EdgeClient.SetFeature(edgeos.PortForwarding, feat["data"])
					break
				}
			}

			for i, rule := range c.FWrules{
				if strings.Contains(rule["description"], "LV-LB-") && 
				rule["forward-to-port"] == strconv.Itoa(int(svc.Spec.Ports[0].NodePort)) {
					c.FWrules = append(c.FWrules[:i], c.FWrules[i+1:]...)
				}
			}


		}
	}
}

func (c *Controller) Controller_loop() {
	for {
		c.Reconcile()
		time.Sleep(2 * time.Second)
	}
}
