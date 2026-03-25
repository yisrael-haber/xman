// vcsim is a local vCenter simulator for development and testing.
// It wraps govmomi's simulator package and exposes configuration via flags.
//
// Default connection details:
//
//	URL:      http://127.0.0.1:8989/sdk
//	Username: user
//	Password: pass
//	TLS:      not used (plain HTTP, local dev only)
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	addr         := flag.String("l", "127.0.0.1:8989", "listen address")
	username     := flag.String("u", "user", "required username")
	password     := flag.String("p", "pass", "required password")
	datacenters  := flag.Int("dc", 1, "number of datacenters")
	clusters     := flag.Int("cluster", 1, "clusters per datacenter")
	clusterHosts := flag.Int("host", 3, "hosts per cluster")
	vms          := flag.Int("vm", 5, "VMs per host")
	datastores   := flag.Int("ds", 1, "datastores per datacenter")
	flag.Parse()

	model := simulator.VPX()
	model.Datacenter  = *datacenters
	model.Cluster     = *clusters
	model.ClusterHost = *clusterHosts
	model.Machine     = *vms
	model.Datastore   = *datastores

	if err := model.Create(); err != nil {
		log.Fatalf("creating model: %v", err)
	}

	// Setting User on the Listen URL causes the simulator to enforce credentials.
	// Without this it accepts any non-empty username/password (simulator default).
	model.Service.Listen = &url.URL{
		Scheme: "http",
		Host:   *addr,
		User:   url.UserPassword(*username, *password),
	}

	server := model.Service.NewServer()

	fmt.Fprintf(os.Stdout, "vcsim listening on http://%s/sdk\n", *addr)
	fmt.Fprintf(os.Stdout, "  username: %s\n", *username)
	fmt.Fprintf(os.Stdout, "  password: %s\n", *password)
	fmt.Fprintf(os.Stdout, "  skip TLS: false (plain HTTP, no TLS)\n")
	fmt.Fprintf(os.Stdout, "  DCs: %d  clusters: %d  hosts/cluster: %d  VMs/host: %d\n\n",
		*datacenters, *clusters, *clusterHosts, *vms)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nShutting down vcsim...")
	server.Close()
	model.Remove()
	fmt.Println("vcsim stopped.")
}
