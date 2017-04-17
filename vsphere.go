package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// GetEnvString returns string from environment variable.
func GetEnvString(v string, def string) string {
	r := os.Getenv(v)
	if r == "" {
		return def
	}

	return r
}

// GetEnvBool returns boolean from environment variable.
func GetEnvBool(v string, def bool) bool {
	r := os.Getenv(v)
	if r == "" {
		return def
	}

	switch strings.ToLower(r[0:1]) {
	case "t", "y", "1":
		return true
	}
	return false
}

const (
	envURL      = "GOVMOMI_URL"
	envUserName = "GOVMOMI_USERNAME"
	envPassword = "GOVMOMI_PASSWORD"
	envInsecure = "GOVMOMI_INSECURE"
)

var urlDescription = fmt.Sprintf("ESX or vCenter URL [%s]", envURL)
var urlFlag = flag.String("url", GetEnvString(envURL, "https://username:password@host/sdk"), urlDescription)

var insecureDescription = fmt.Sprintf("Don't verify the server's certificate chain [%s]", envInsecure)
var insecureFlag = flag.Bool("insecure", GetEnvBool(envInsecure, false), insecureDescription)

func exit(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

func GatherDataStoreMetrics(ctx context.Context, c *govmomi.Client, pc *property.Collector, dss []*object.Datastore) {
	// Convert datastores into list of references
	var refs []types.ManagedObjectReference
	for _, ds := range dss {
		refs = append(refs, ds.Reference())
	}

	// Retrieve summary property for all datastores
	var dst []mo.Datastore
	err := pc.Retrieve(ctx, refs, []string{"summary"}, &dst)
	if err != nil {
		exit(err)
	}

	for _, ds := range dst {

		records := make(map[string]interface{})
		tags := make(map[string]string)

		tags["name"] = ds.Summary.Name
		tags["type"] = ds.Summary.Type
		tags["url"] = ds.Summary.Url

		records["capacity"] = ds.Summary.Capacity
		records["freespace"] = ds.Summary.FreeSpace
	}
}

func GatherVMMetrics(ctx context.Context, c *govmomi.Client, pc *property.Collector, vms []*object.VirtualMachine) {
	// Convert datastores into list of references
	var refs []types.ManagedObjectReference
	for _, vm := range vms {
		refs = append(refs, vm.Reference())
	}

	// Retrieve name property for all vms
	var vmt []mo.VirtualMachine
	err := pc.Retrieve(ctx, refs, []string{"name", "config", "summary"}, &vmt)
	if err != nil {
		exit(err)
	}

	for _, vm := range vmt {

		records := make(map[string]interface{})
		tags := make(map[string]string)

		tags["name"] = vm.Name
		tags["guest_full_name"] = vm.Config.GuestFullName
		tags["connection_state"] = vm.Summary.Runtime.ConnectionState
		tags["overall_status"] = vm.Summary.OverallStatus
		tags["vm_path_name"] = vm.Summary.Config.VmPathName
		tags["ip_address"] = vm.Summary.Guest.IpAddress
		tags["hostname"] = vm.Summary.Guest.HostName
		tags["guest_id"] = vm.Config.GuestId
		tags["is_guest_tools_running"] = vm.Summary.Guest.ToolsRunningStatus

		records["mem_mb"] = vm.Config.Hardware.MemoryMB
		records["num_cpu"] = vm.Config.Hardware.NumCPU
		records["host_mem_usage"] = vm.Summary.QuickStats.HostMemoryUsage
		records["guest_mem_usage"] = vm.Summary.QuickStats.GuestMemoryUsage
		records["overall_cpu_usage"] = vm.Summary.QuickStats.OverallCpuUsage
		records["overall_cpu_demand"] = vm.Summary.QuickStats.OverallCpuDemand
		records["swap_mem"] = vm.Summary.QuickStats.SwappedMemory
		records["uptime_sec"] = vm.Summary.QuickStats.UptimeSeconds
		records["storage_committed"] = vm.Summary.Storage.Committed
		records["storage_uncommitted"] = vm.Summary.Storage.Uncommitted
		records["max_cpu_usage"] = vm.Summary.Runtime.MaxCpuUsage
		records["max_mem_usage"] = vm.Summary.Runtime.MaxMemoryUsage
		records["num_cores_per_socket"] = vm.Config.Hardware.NumCoresPerSocket

	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//
	flag.Parse()

	// Parse URL from string
	u, err := url.Parse(os.Getenv("GOVMOMI_URL"))
	if err != nil {
		exit(err)
	}

	// Connect and log in to ESX or vCenter
	c, err := govmomi.NewClient(ctx, u, string(os.Getenv("GOVMOMI_INSECURE")))
	if err != nil {
		exit(err)
	}
	f := find.NewFinder(c.Client, true)

	// Find one and only datacenter
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		exit(err)
	}

	// Make future calls local to this datacenter
	f.SetDatacenter(dc)

	pc := property.DefaultCollector(c.Client)

	dss, err := f.DatastoreList(ctx, "*")
	if err != nil {
		exit(err)
	}

	GatherDataStoreMetrics(ctx, c, pc, dss)

	// Find virtual machines in datacenter
	vms, err := f.VirtualMachineList(ctx, "*")
	if err != nil {
		exit(err)
	}
	GatherVMMetrics(ctx, c, pc, vms)

}
