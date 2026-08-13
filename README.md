# Overwatch server picker for linux

A simple CLI overwatch server selector on Linux.

### Dependencies
* `golang`
* `nftables` OR `iptables` + `ipset`
  
### Usage
```sudo go run main.go```

### Credits
IP retrieving logic was taken from [stowmyy/dropship](https://github.com/stowmyy/dropship)

### Contributing
Feel free to contribute! Maybe you'll find a universal ping logic without hardcoding ping servers - who knows?
