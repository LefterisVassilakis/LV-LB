package controller

import (
	//"reflect"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
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
	Clientset      *kubernetes.Clientset
	Context        context.Context
	EdgeClient     *edgeos.Client
	state 			bool
	RouterIp 	   string
	NodeIp         string
}

func New(clientset *kubernetes.Clientset, ctx context.Context, int_st bool, NodeIp string) *Controller {
	return &Controller{
		Clientset:      clientset,
		Context:        ctx,
		EdgeClient:     nil,
		state: int_st,
		RouterIp: 	   "",
		NodeIp:         NodeIp,
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

	c.RouterIp = addr
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
	return strings.Contains(description, "LV-LB-"+svcName+"-"+port)
}

func empty_rules(feat map[string]interface{}) bool {
	data, dataExists := feat["data"].(map[string]interface{})
	if !dataExists || data == nil {
		return true
	}
	
	rulesConfig, rulesConfigExists := data["rules-config"]
	if !rulesConfigExists || rulesConfig == nil || rulesConfig == "" {
		return true
	}

	return false
}

func (c *Controller) listFW() []map[string]string {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	for empty_rules(feat){
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(2*time.Second)
		feat, err = c.EdgeClient.Feature(edgeos.PortForwarding)
	}

	rawRules := feat["data"].(map[string]interface{})["rules-config"].([]interface{})

	var rules []map[string]string
	for _, rawRule := range rawRules {
		ruleMap, ok := rawRule.(map[string]interface{})
		if !ok {
			continue
		}

		stringRule := make(map[string]string)
		for k, v := range ruleMap {
			if str, ok := v.(string); ok {
				stringRule[k] = str
			} else {
				fmt.Printf("Warning: Unexpected value type for key %q\n", k)
			}
		}
		rules = append(rules, stringRule)
	}

	return rules
}

func data_contains(data []interface{}, item map[string]string) bool {
	for _, d := range data {
		d := d.(map[string]interface{})
		if d["description"] == item["description"] && d["forward-to-port"] == item["forward-to-port"] &&
			d["forward-to-address"] == item["forward-to-address"] && d["protocol"] == item["protocol"] &&
			d["original-port"] == item["original-port"] {
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
				rule["forward-to-address"] == c.NodeIp && rule["original-port"] == strconv.Itoa(int(port.NodePort)) {
				return true
			}
		}
	}
	return false
}

func (c *Controller) addIPtoLBsvc(svc corev1.Service) {
	newIngress := v1.LoadBalancerIngress{
		IP: strings.TrimPrefix(c.RouterIp, "https://"),
	}

	svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, newIngress)

	_, err := c.Clientset.CoreV1().Services(svc.Namespace).UpdateStatus(context.TODO(), &svc, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}

}

func (c *Controller) add_FWrule(svc corev1.Service) {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	for empty_rules(feat){
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(2*time.Second)
		feat, err = c.EdgeClient.Feature(edgeos.PortForwarding)
	}

	d := feat["data"].(map[string]interface{})["rules-config"].([]interface{})
	newFWrule := false

	for i, _ := range svc.Spec.Ports {

		portForwards := make(map[string]string)
		portForwards["description"] = "LV-LB-" + svc.Name + "-" + strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["forward-to-address"] = c.NodeIp
		portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
		portForwards["protocol"] = "tcp"

		if !data_contains(d, portForwards) {

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
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	return err
}

func loadData(filename string) ([]map[string]string, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	jsonData, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if len(jsonData) == 0 {
		return []map[string]string{}, nil
	}

	var data []map[string]string
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func FWrules_to_delete(oldFWrules []map[string]string, currFWrules []map[string]string) []map[string]string {
	rule_to_delete := []map[string]string{}
	for i, old_rule := range oldFWrules {
		found := false
		for _, curr_rule := range currFWrules {
			if old_rule["description"] == curr_rule["description"] && old_rule["forward-to-port"] == curr_rule["forward-to-port"] &&
				old_rule["forward-to-address"] == curr_rule["forward-to-address"] && old_rule["protocol"] == curr_rule["protocol"] &&
				old_rule["original-port"] == curr_rule["original-port"] {
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

func (c *Controller) FWrules_need() []map[string]string {
	FWrules_needed := []map[string]string{}
	services := c.listSVC()
	for _, svc := range services.Items {
		for i, _ := range svc.Spec.Ports {
			portForwards := make(map[string]string)
			portForwards["description"] = "LV-LB-" + svc.Name + "-" + strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["forward-to-address"] = c.NodeIp
			portForwards["forward-to-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["original-port"] = strconv.Itoa(int(svc.Spec.Ports[i].NodePort))
			portForwards["protocol"] = "tcp"

			FWrules_needed = append(FWrules_needed, portForwards)
		}
	}
	return FWrules_needed
}

func (c *Controller) Delete_FWrules(rules []map[string]string) {
	feat, err := c.EdgeClient.Feature(edgeos.PortForwarding)
	for empty_rules(feat){
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(2*time.Second)
		feat, err = c.EdgeClient.Feature(edgeos.PortForwarding)
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

func (c *Controller) remove_other_rules(rules []map[string]string) []map[string]string {
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
			if !has_FWrule {
				update = true
				c.addIPtoLBsvc(svc)
				c.add_FWrule(svc)
			}
		}
	}
	neededFWrules := c.FWrules_need()
	if c.state { // persistent state
		oldFWrules, err := loadData("state.json")
		if err != nil {
			log.Fatal(err)
		}
		rules_to_delete := FWrules_to_delete(oldFWrules, neededFWrules)
		if len(rules_to_delete) > 0 {
			c.Delete_FWrules(rules_to_delete)
			update = true
		}

		if update {
			err := saveData(neededFWrules, "state.json")
			if err != nil {
				log.Fatal(err)
			}
		}
	} else { // no state
		oldFWrules := c.listFW()
		rules_to_delete := FWrules_to_delete(oldFWrules, neededFWrules)
		rules_to_delete = c.remove_other_rules(rules_to_delete)
		if len(rules_to_delete) > 0 {
			c.Delete_FWrules(rules_to_delete)
		}
	}
}

func (c *Controller) Controller_loop() {
	if c.EdgeClient == nil {
		log.Fatal("EdgeClient is not connected")
	}
	for {
		c.Reconcile()
		time.Sleep(2 * time.Second)
	}
}
