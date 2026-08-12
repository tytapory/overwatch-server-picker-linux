package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"
)

const (
	ipsetNameIPv4 = "OW_SERVERS_BLOCKED_IPV4"
	ipsetNameIPv6 = "OW_SERVERS_BLOCKED_IPV6"
	nftTableName  = "OW_SERVERS_BLOCKED"

	nft       = "nft"
	iptables  = "iptables"
	ip6tables = "ip6tables"
	ipset     = "ipset"
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

	cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigChan
		cleanup()

		os.Exit(0)
	}()

	fmt.Print("\033cSelect mode:\n1) Auto\n2) Manual\n")
	var selection int
	fmt.Scanf("%d", &selection)
	for selection < 1 || selection > 2 {
		fmt.Printf("\033cInvalid choice: %d\nSelect mode:\n1) Auto\n2) Manual\n", selection)
		fmt.Scanf(" %d", &selection)
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

	cleanup()
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
			if selection < 1 || selection > len(servers) {
				fmt.Printf("Invalid choice: %d. ", selection)
			} else {
				servers[selection-1].IsBlocked = true
			}

			selection = 0
		case 2:
			for i := range servers {
				servers[i].IsBlocked = true
			}
		case 3:
			fmt.Print("Select server to unblock\n\n")
			printServers(servers)

			fmt.Scanf("%d", &selection)

			fmt.Print("\033c")
			if selection < 1 || selection > len(servers) {
				fmt.Printf("Invalid choice: %d. ", selection)
			} else {
				servers[selection-1].IsBlocked = false
			}

			selection = 0
		case 4:
			for i := range servers {
				servers[i].IsBlocked = false
			}
		case 5:
		default:
			fmt.Printf("Invalid choice: %d. ", selection)
		}
	}

	return runService(servers)
}

func runService(servers []Server) error {
	if _, err := exec.LookPath("nft"); err == nil {
		return runServiceNft(servers)
	}

	_, errIPT := exec.LookPath("iptables")
	_, errSet := exec.LookPath("ipset")

	if errIPT == nil && errSet == nil {
		return runServiceIptables(servers)
	}

	if errIPT == nil && errSet != nil {
		return fmt.Errorf("iptables found but ipset not found")
	}

	return fmt.Errorf("neither nftables nor iptables+ipset found")
}

func runServiceNft(servers []Server) error {
	err := runCmd(nft, "add", "table", "inet", nftTableName)
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "chain", "inet", nftTableName, "output", "{", "type", "filter", "hook", "output", "priority", "0", ";", "policy", "accept", ";", "}")
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "set", "inet", nftTableName, ipsetNameIPv4, "{", "type", "ipv4_addr", ";", "flags", "interval", ";", "}")
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "set", "inet", nftTableName, ipsetNameIPv6, "{", "type", "ipv6_addr", ";", "flags", "interval", ";", "}")
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "rule", "inet", nftTableName, "output", "ip", "daddr", "@"+ipsetNameIPv4, "drop")
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "rule", "inet", nftTableName, "output", "ip6", "daddr", "@"+ipsetNameIPv6, "drop")
	if err != nil {
		return err
	}

	var (
		ipv4Servers strings.Builder
		ipv6Servers strings.Builder
	)

	for _, server := range servers {
		if len(server.IPv4) > 0 {
			for _, ip := range server.IPv4 {
				if ipv4Servers.Len() > 0 {
					ipv4Servers.WriteByte(',')
				}
				ipv4Servers.WriteString(ip)
			}
		}

		if len(server.IPv6) > 0 {
			for _, ip := range server.IPv6 {
				if ipv6Servers.Len() > 0 {
					ipv6Servers.WriteByte(',')
				}
				ipv6Servers.WriteString(ip)
			}
		}
	}

	err = runCmd(nft, "add", "element", "inet", nftTableName, ipsetNameIPv4, "{", ipv4Servers.String(), "}")
	if err != nil {
		return err
	}

	err = runCmd(nft, "add", "element", "inet", nftTableName, ipsetNameIPv6, "{", ipv6Servers.String(), "}")
	if err != nil {
		return err
	}

	fmt.Println("service running. press ctrl + c to to exit")

	select {}
}

func runServiceIptables(servers []Server) error {
	err := runCmd(ipset, "create", ipsetNameIPv4, "hash:net", "family", "inet", "-exist")
	if err != nil {
		return err
	}

	err = runCmd(ipset, "create", ipsetNameIPv6, "hash:net", "family", "inet6", "-exist")
	if err != nil {
		return err
	}

	err = runCmd(iptables, "-I", "OUTPUT", "-m", "set", "--match-set", ipsetNameIPv4, "dst", "-j", "DROP")
	if err != nil {
		return err
	}

	err = runCmd(ip6tables, "-I", "OUTPUT", "-m", "set", "--match-set", ipsetNameIPv6, "dst", "-j", "DROP")
	if err != nil {
		return err
	}

	for _, server := range servers {
		for _, ip := range server.IPv4 {
			err = runCmd(ipset, "add", ipsetNameIPv4, ip, "-exist")
			if err != nil {
				return err
			}
		}

		for _, ip := range server.IPv6 {
			err = runCmd(ipset, "add", ipsetNameIPv6, ip, "-exist")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v exec error: %s %s", name, args, err.Error(), string(out))
	}

	return nil
}

func cleanup() {
	runCmd(nft, "delete", "table", "inet", nftTableName)
	runCmd(iptables, "-D", "OUTPUT", "-m", "set", "--match-set", ipsetNameIPv4, "dst", "-j", "DROP")
	runCmd(ip6tables, "-D", "OUTPUT", "-m", "set", "--match-set", ipsetNameIPv6, "dst", "-j", "DROP")
	runCmd(ipset, "destroy", ipsetNameIPv4)
	runCmd(ipset, "destroy", ipsetNameIPv6)
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
