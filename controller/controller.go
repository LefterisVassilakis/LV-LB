package controller

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"encoding/json"
	"io"
	"os"
	"regexp"

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
	internal_state bool
}

func New(clientset *kubernetes.Clientset, ctx context.Context, int_st bool) *Controller {
	return &Controller{
		Clientset:  clientset,
		Context:    ctx,
		LBservices: []corev1.Service{},
		FWrules:    []map[string]string{},
		EdgeClient: nil,
		internal_state: int_st,
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

func description_equal(description string, svcName string, port string) bool {
	return strings.Contains(description, "LV-LB-" + svcName + "-" + port)
}

func (c *Controller) listFW() []map[string]string {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	rawRules := feat["data"].(map[string]interface{})["rules-config"].([]interface{})

    var rules []map[string]string
    for _, rawRule := range rawRules {
        ruleMap, ok := rawRule.(map[string]interface{})
        if !ok {
            // Handle the case where the element is not of type map[string]interface{}
            continue
        }

        stringRule := make(map[string]string)
        for k, v := range ruleMap {
            if str, ok := v.(string); ok {
                stringRule[k] = str
            } else {
                // Handle the case where the value is not of type string
                // You might want to log a warning or handle it differently based on your requirements
                fmt.Printf("Warning: Unexpected value type for key %q\n", k)
            }
        }

        // Append the converted rule to the rules slice
        rules = append(rules, stringRule)
    }

    return rules
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

func data_contains(data []interface{}, item map[string]string) bool {
	for _, d := range data {
		d := d.(map[string]interface{})
		if d["description"] == item["description"] && d["forward-to-port"] == item["forward-to-port"] && 
		d["forward-to-address"] == item["forward-to-address"] && d["protocol"] == item["protocol"] &&
		d["original-port"] == item["original-port"]{
			return true
		}
	}
	return false
}

func (c *Controller) LBsvc_has_FWrule(svc corev1.Service) bool {
	FWrules := c.listFW()
	for _, port := range svc.Spec.Ports {
		for _, rule := range FWrules {
			if description_equal(rule["description"], svc.Name, strconv.Itoa(int(port.NodePort))) && rule["forward-to-port"] == strconv.Itoa(int(port.NodePort)) &&
			rule["forward-to-address"] == "192.168.1.108" && rule["original-port"] == strconv.Itoa(int(port.NodePort)) {
				return true
			}
		}
	}
	return false
}



func (c *Controller) addIPtoLBsvc(svc corev1.Service) {
	newIngress := v1.LoadBalancerIngress{
		IP:       "139.91.92.131",
	}

	svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, newIngress)

	_, err := c.Clientset.CoreV1().Services(svc.Namespace).UpdateStatus(context.TODO(), &svc, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}

}

func (c *Controller) add_FWrule(svc corev1.Service) {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	d := feat["data"].(map[string]interface{})["rules-config"].([]interface{})
	newFWrule := false

	for i, _ := range svc.Spec.Ports {

		portForwards := make(map[string]string)
		portForwards["description"] = "LV-LB-" + svc.Name + "-" + strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["forward-to-address"] = "192.168.1.108"
		portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["protocol"] = "tcp"

		if !data_contains(d, portForwards) {

			// fmt.Println("New port forwarding rule: ", portForwards)

			newFWrule = true
			d = append(d, portForwards)

		}
	}

	if newFWrule { // Check if FW rules have changed
		feat["data"].(map[string]interface{})["rules-config"] = d
		c.EdgeClient.SetFeature(edgeos.PortForwarding, feat["data"])
	}	
}

func saveData(data []map[string]string, filename string) error {
    // Marshal the list of maps to JSON
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    // Write the JSON data to a file
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    _, err = file.Write(jsonData)
    return err
}

func loadData(filename string) ([]map[string]string, error) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // Read the JSON data from the file
    jsonData, err := io.ReadAll(file)
    if err != nil {
        return nil, err
    }

	if len(jsonData) == 0 {
		return []map[string]string{}, nil
	}
    
    // Unmarshal the JSON data into a list of maps
    var data []map[string]string
    err = json.Unmarshal(jsonData, &data)
    if err != nil {
        return nil, err
    }
    
    return data, nil
}

func FWrules_to_delete(oldFWrules []map[string]string, currFWrules []map[string]string) []map[string]string{
	rule_to_delete := []map[string]string{}
	for i, old_rule := range oldFWrules {
		found := false
		for _, curr_rule := range currFWrules {
			if old_rule["description"] == curr_rule["description"] && old_rule["forward-to-port"] == curr_rule["forward-to-port"] && 
			old_rule["forward-to-address"] == curr_rule["forward-to-address"] && old_rule["protocol"] == curr_rule["protocol"] &&
			old_rule["original-port"] == curr_rule["original-port"]{
				found = true
				break
			}
		}
		if !found {
			rule_to_delete = append(rule_to_delete, oldFWrules[i])
		}
	}
	return rule_to_delete
}
	
func (c *Controller) FWrules_need() []map[string]string{
	FWrules_needed := []map[string]string{}
	services := c.listSVC()
	for _, svc := range services.Items {
		for i, _ := range svc.Spec.Ports {
			portForwards := make(map[string]string)
			portForwards["description"] = "LV-LB-" + svc.Name + "-" + strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["forward-to-address"] = "192.168.1.108"
			portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["protocol"] = "tcp"

			FWrules_needed = append(FWrules_needed, portForwards)
		}
	}
	return FWrules_needed
}

func (c *Controller) delete_FWrules(rules []map[string]string) {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	d := feat["data"].(map[string]interface{})["rules-config"].([]interface{})

	for _, rule := range rules {
		for i, _ := range d {
			if d[i].(map[string]interface{})["description"] == rule["description"] && d[i].(map[string]interface{})["forward-to-port"] == rule["forward-to-port"] && 
			d[i].(map[string]interface{})["forward-to-address"] == rule["forward-to-address"] && d[i].(map[string]interface{})["protocol"] == rule["protocol"] &&
			d[i].(map[string]interface{})["original-port"] == rule["original-port"] {
				d = append(d[:i], d[i+1:]...)
				break
			}
		}
	}

	feat["data"].(map[string]interface{})["rules-config"] = d
	c.EdgeClient.SetFeature(edgeos.PortForwarding, feat["data"])
}

func (c *Controller) is_my_rule(rule map[string]string) bool {
	pattern := `^LV-LB-[a-zA-Z0-9\-]+-\d+$`
	re := regexp.MustCompile(pattern)

	return re.MatchString(rule["description"])
}

func (c *Controller) remove_other_rules(rules []map[string]string) []map[string]string{
	for i, rule := range rules {
		if !c.is_my_rule(rule) {
			rules = append(rules[:i], rules[i+1:]...)
		}
	}
	return rules
}


func (c *Controller) Reconcile() {
	update := false
	// Observe new LB services
	services := c.listSVC()
	for _, svc := range services.Items {
		if svc.Spec.Type == "LoadBalancer" {
			has_FWrule := c.LBsvc_has_FWrule(svc)
			// fmt.Printf("Service %s has FW rule: %v\n", svc.Name, has_FWrule)
			if !has_FWrule {
				update = true
				c.addIPtoLBsvc(svc)
				c.add_FWrule(svc)
				has_FWrule = true
			}
		}
	}

	neededFWrules := c.FWrules_need()

	if c.internal_state {
		oldFWrules, err := loadData("backup.json")
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Printf("Current FW rules: %v\n\n", neededFWrules)
		rules_to_delete := FWrules_to_delete(oldFWrules, neededFWrules)
		// fmt.Printf("Rules to delete: %v\n", rules_to_delete)

		// delete rules from router and make update true
		if len(rules_to_delete) > 0 {
			c.delete_FWrules(rules_to_delete)
			update = true
		}

		if update {
			err := saveData(neededFWrules, "backup.json")
			if err != nil {
				log.Fatal(err)
			}
		}
	}else{ // no internal state
		oldFWrules := c.listFW()

		rules_to_delete := FWrules_to_delete(oldFWrules, neededFWrules)
		rules_to_delete = c.remove_other_rules(rules_to_delete)

		if len(rules_to_delete) > 0 {
			c.delete_FWrules(rules_to_delete)
		}
	}
}

func (c *Controller) Controller_loop() {
	for {
		c.Reconcile()
		time.Sleep(2 * time.Second)
	}
}
