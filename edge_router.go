package main

import (
	"log"

	"astuart.co/edgeos-rest/pkg/edgeos"
)

func test() {
	c, err := edgeos.NewClient("https://192.168.1.1", "ubnt", "raspberryk8s")

	if err != nil {
		log.Fatal(err)
	}

	if err := c.Login(); err != nil {
		log.Fatal(err)
	}

	feat, err := c.Feature(edgeos.PortForwarding)
	if err != nil {
		log.Fatal(err)
	}

	// forwardingRule := edgeos.PortForward{
	// 	PortFrom:    "original-port:80",
	// 	PortTo:      "forward-to-port:80",
	// 	IPTo:        "forward-to-address:192.168.1.2",
	// 	Protocol:    "protocol:tcp",
	// 	Description: "description:program_test",
	// }

	// portForwards := edgeos.PortForwards{
	// 	Rules: []edgeos.PortForward{forwardingRule},
	// }

    portForwards := make(map[string]string)
    portForwards["description"] = "program_test"
    portForwards["forward-to-address"] = "192.168.1.2"
    portForwards["forward-to-port"] = "80"
    portForwards["original-port"] = "80"
    portForwards["protocol"] = "tcp"


	d := feat["data"].(map[string]interface{})["rules-config"].([]interface{})

	d = append(d, portForwards)

    feat["data"].(map[string]interface{})["rules-config"] = d

    log.Println(feat["data"])

	log.Println(c.SetFeature(edgeos.PortForwarding, feat["data"]))

}
