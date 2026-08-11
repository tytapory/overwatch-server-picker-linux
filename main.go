package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
)

type Server struct {
	Name string   `json:"name"`
	IPv4 []net.IP `json:"ipv4,omitempty"`
	IPv6 []net.IP `json:"ipv6,omitempty"`
}

type GooglePrefixes struct {
	Prefixes []GooglePrefix `json:"prefixes"`
}

type GooglePrefix struct {
	IPv4Prefix net.IP `json:"ipv4Prefix,omitempty"`
	IPv6Prefix net.IP `json:"ipv6Prefix,omitempty"`
	Scope      string `json:"scope"`
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("you have to run as root")

		return
	}

	fmt.Print("\033cSelect mode:\n1) Auto\n2) Manual\n")
	var selection int
	fmt.Scanf("%d", &selection)
	for selection < 1 || selection > 2 {
		fmt.Printf("\033cInvalid choice: %d\nSelect mode:\n1) Auto\n2) Manual\n", selection)
		fmt.Scanf("%d", &selection)
	}

}

func auto() {

}

func manual() {

}

func getServers() error {
	bytes, err := os.ReadFile("ips.json")
	if err != nil {
		return err
	}

	var servers []Server
	err = json.Unmarshal(bytes, &servers)
	if err != nil {
		return err
	}

	resp, err := http.Get("https://www.gstatic.com/ipranges/cloud.json")
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var googlePrefixes GooglePrefixes
	err = json.NewDecoder(resp.Body).Decode(&googlePrefixes)
	if err != nil {
		return err
	}

	for _, prefix := range googlePrefixes.Prefixes {
		temp := Server{
			IPv4: []net.IP{prefix.IPv4Prefix},
			IPv6: []net.IP{prefix.IPv6Prefix},
		}
		switch prefix.Scope {
		case "southamerica-east1":

		}
	}
}
