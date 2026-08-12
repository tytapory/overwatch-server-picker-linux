package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
)

type Server struct {
	Name      string   `json:"name"`
	IPv4      []string `json:"ipv4,omitempty"`
	IPv6      []string `json:"ipv6,omitempty"`
	IsBlocked bool
}

type GooglePrefixes struct {
	Prefixes []GooglePrefix `json:"prefixes"`
}

type GooglePrefix struct {
	IPv4Prefix string `json:"ipv4Prefix,omitempty"`
	IPv6Prefix string `json:"ipv6Prefix,omitempty"`
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

	var err error
	if selection == 1 {
		err = auto()
	} else {
		err = manual()
	}

	if err != nil {
		fmt.Println(err.Error())
	}
}

func auto() error {
	result, err := getServers()
	fmt.Println(result)
	return err
}

func manual() error {
	servers, err := getServers()
	if err != nil {
		return err
	}

	selection := 0

	fmt.Print("\033c")
	for selection != 5 {
		fmt.Print("Select action:\n\n1) Block\n2) Block all\n3) Unblock\n4) Unblock all\n5) Start service\n\n\n")

		fmt.Print("Current servers:\n\n")
		printServers(servers)

		fmt.Scanf("%d", &selection)

		fmt.Print("\033c")
		switch selection {
		case 1:
			fmt.Print("Select server to block\n\n")
			printServers(servers)

			fmt.Scanf("%d", &selection)

			fmt.Print("\033c")
			if selection < 1 || selection > len(servers)+1 {
				fmt.Printf("Invalid choice: %d. ", selection)
			} else {
				servers[selection-1].IsBlocked = true
			}
		case 2:
			for i := range servers {
				servers[i].IsBlocked = true
			}
		case 3:
			fmt.Print("Select server to unblock\n\n")
			printServers(servers)

			fmt.Scanf("%d", &selection)

			fmt.Print("\033c")
			if selection < 1 || selection > len(servers)+1 {
				fmt.Printf("Invalid choice: %d. ", selection)
			} else {
				servers[selection-1].IsBlocked = false
			}
		case 4:
			for i := range servers {
				servers[i].IsBlocked = false
			}
		case 5:
		default:
			fmt.Printf("Invalid choice: %d. ", selection)
		}
	}

	return nil
}

func printServers(servers []Server) {
	for i, server := range servers {
		fmt.Printf("%d) %s: ", i+1, server.Name)
		if server.IsBlocked {
			fmt.Println("Blocked")
		} else {
			fmt.Println("Available")
		}
	}
}

func getServers() ([]Server, error) {
	bytes, err := os.ReadFile("ips.json")
	if err != nil {
		return nil, err
	}

	var servers []Server
	err = json.Unmarshal(bytes, &servers)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get("https://www.gstatic.com/ipranges/cloud.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var googlePrefixes GooglePrefixes
	err = json.NewDecoder(resp.Body).Decode(&googlePrefixes)
	if err != nil {
		return nil, err
	}

	bytes, err = os.ReadFile("googleservers.json")
	if err != nil {
		return nil, err
	}

	var googleServersMap map[string]string
	err = json.Unmarshal(bytes, &googleServersMap)
	if err != nil {
		return nil, err
	}

	serversMap := make(map[string]Server)
	for _, prefix := range googlePrefixes.Prefixes {
		if googleServer, ok := googleServersMap[prefix.Scope]; ok {
			temp := serversMap[prefix.Scope]
			temp.Name = googleServer
			if prefix.IPv4Prefix != "" {
				temp.IPv4 = append(temp.IPv4, prefix.IPv4Prefix)
			}
			if prefix.IPv6Prefix != "" {
				temp.IPv6 = append(temp.IPv6, prefix.IPv6Prefix)
			}
			serversMap[prefix.Scope] = temp
		}
	}

	for _, server := range serversMap {
		servers = append(servers, server)
	}

	slices.SortFunc(servers, func(a Server, b Server) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return servers, nil
}
