package controller

import (
    "context"
	"log"

    "k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    corev1 "k8s.io/api/core/v1"
	"astuart.co/edgeos-rest/pkg/edgeos"
)

type Controller struct{
	Clientset *kubernetes.Clientset
	Context context.Context
	LBservices []corev1.Service
	FWrules []edgeos.PortForward
}

func New(clientset *kubernetes.Clientset, ctx context.Context) *Controller {
	return &Controller{
		Clientset: clientset,
		Context: ctx,
		LBservices: []corev1.Service{},
		FWrules: []edgeos.PortForward{},
	}
}

func (c *Controller) listSVC() *corev1.ServiceList {
	services, err := c.Clientset.CoreV1().Services("").List(c.Context, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing services: %v", err)
	}
	return services
}

func (c *Controller) reconcile() {
	services := c.listSVC()
	for _, svc := range services.Items {
		log.Println(svc)
		// log.Printf("Service: %s", svc.Name)
		// if svc.Spec.Type == "LoadBalancer" {
		// 	c.LBservices = append(c.LBservices, svc)
		// }
	}
}