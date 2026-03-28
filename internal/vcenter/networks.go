package vcenter

import (
	"context"
	"fmt"
	"sort"

	"xman/internal/manager"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func (b *Backend) ListNetworks(ctx context.Context) (manager.NetworkSummary, error) {
	client, err := b.session.Client()
	if err != nil {
		return manager.NetworkSummary{}, err
	}

	viewManager := view.NewManager(client.Client)
	hosts, err := listNetworkHosts(ctx, viewManager, client.ServiceContent.RootFolder)
	if err != nil {
		return manager.NetworkSummary{}, err
	}

	hostNames, standardSwitches, standardPortGroups := buildStandardNetworkTopology(hosts)

	dvSwitches, err := listDistributedSwitches(ctx, viewManager, client.ServiceContent.RootFolder)
	if err != nil {
		return manager.NetworkSummary{}, err
	}
	distributedSwitches := buildDistributedSwitchTopology(dvSwitches)

	dvPortGroups, err := listDistributedPortGroups(ctx, viewManager, client.ServiceContent.RootFolder)
	if err != nil {
		return manager.NetworkSummary{}, err
	}
	attachDistributedPortGroups(distributedSwitches, dvPortGroups, hostNames)

	return manager.NetworkSummary{
		Switches: assembleNetworkSwitches(standardSwitches, standardPortGroups, distributedSwitches),
	}, nil
}

func listNetworkHosts(ctx context.Context, viewManager *view.Manager, root types.ManagedObjectReference) ([]mo.HostSystem, error) {
	hostView, err := viewManager.CreateContainerView(ctx, root, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("host view: %w", err)
	}
	defer hostView.Destroy(ctx)

	var hosts []mo.HostSystem
	if err := hostView.Retrieve(ctx, []string{"HostSystem"}, []string{
		"name",
		"config.network.vswitch",
		"config.network.portgroup",
	}, &hosts); err != nil {
		return nil, fmt.Errorf("fetching hosts: %w", err)
	}
	return hosts, nil
}

func listDistributedSwitches(ctx context.Context, viewManager *view.Manager, root types.ManagedObjectReference) ([]mo.VmwareDistributedVirtualSwitch, error) {
	switchView, err := viewManager.CreateContainerView(ctx, root, []string{"VmwareDistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("dvs view: %w", err)
	}
	defer switchView.Destroy(ctx)

	var switches []mo.VmwareDistributedVirtualSwitch
	if err := switchView.Retrieve(ctx, []string{"VmwareDistributedVirtualSwitch"}, []string{
		"name", "config",
	}, &switches); err != nil {
		return nil, fmt.Errorf("fetching dvs: %w", err)
	}
	return switches, nil
}

func listDistributedPortGroups(ctx context.Context, viewManager *view.Manager, root types.ManagedObjectReference) ([]mo.DistributedVirtualPortgroup, error) {
	portGroupView, err := viewManager.CreateContainerView(ctx, root, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, fmt.Errorf("dvpg view: %w", err)
	}
	defer portGroupView.Destroy(ctx)

	var portGroups []mo.DistributedVirtualPortgroup
	if err := portGroupView.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{
		"name", "config", "host", "vm",
	}, &portGroups); err != nil {
		return nil, fmt.Errorf("fetching dvpgs: %w", err)
	}
	return portGroups, nil
}

func buildStandardNetworkTopology(hosts []mo.HostSystem) (map[string]string, map[string]*manager.SwitchInfo, map[string]map[string]*manager.PortGroupInfo) {
	hostNames := make(map[string]string, len(hosts))
	switches := make(map[string]*manager.SwitchInfo)
	portGroups := make(map[string]map[string]*manager.PortGroupInfo)

	for _, host := range hosts {
		hostName := host.Name
		hostNames[host.Reference().Value] = hostName

		if host.Config == nil || host.Config.Network == nil {
			continue
		}

		for _, vswitch := range host.Config.Network.Vswitch {
			switchInfo := switches[vswitch.Name]
			if switchInfo == nil {
				switchInfo = &manager.SwitchInfo{Name: vswitch.Name, Type: "standard", MTU: vswitch.Mtu}
				switches[vswitch.Name] = switchInfo
			}
			switchInfo.Hosts = manager.AppendUnique(switchInfo.Hosts, hostName)
			if bridge, ok := vswitch.Spec.Bridge.(*types.HostVirtualSwitchBondBridge); ok {
				for _, nic := range bridge.NicDevice {
					switchInfo.Uplinks = manager.AppendUnique(switchInfo.Uplinks, nic)
				}
			}
		}

		for _, portGroup := range host.Config.Network.Portgroup {
			bySwitch := portGroups[portGroup.Spec.VswitchName]
			if bySwitch == nil {
				bySwitch = make(map[string]*manager.PortGroupInfo)
				portGroups[portGroup.Spec.VswitchName] = bySwitch
			}

			portGroupInfo := bySwitch[portGroup.Spec.Name]
			if portGroupInfo == nil {
				portGroupInfo = &manager.PortGroupInfo{
					Name: portGroup.Spec.Name,
					VLAN: manager.FormatVLAN(portGroup.Spec.VlanId),
				}
				bySwitch[portGroup.Spec.Name] = portGroupInfo
			}
			portGroupInfo.Hosts = manager.AppendUnique(portGroupInfo.Hosts, hostName)
		}
	}

	return hostNames, switches, portGroups
}

func buildDistributedSwitchTopology(dvSwitches []mo.VmwareDistributedVirtualSwitch) map[string]*manager.SwitchInfo {
	switches := make(map[string]*manager.SwitchInfo, len(dvSwitches))
	for i := range dvSwitches {
		dvSwitch := &dvSwitches[i]
		switchInfo := &manager.SwitchInfo{Name: dvSwitch.Name, Type: "distributed"}
		if cfg, ok := dvSwitch.Config.(*types.VMwareDVSConfigInfo); ok {
			switchInfo.MTU = cfg.MaxMtu
			if policy, ok := cfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
				switchInfo.Uplinks = append([]string(nil), policy.UplinkPortName...)
			}
		}
		switches[dvSwitch.Reference().Value] = switchInfo
	}
	return switches
}

func attachDistributedPortGroups(switches map[string]*manager.SwitchInfo, portGroups []mo.DistributedVirtualPortgroup, hostNames map[string]string) {
	for _, portGroup := range portGroups {
		if portGroup.Config.DistributedVirtualSwitch == nil {
			continue
		}

		switchInfo := switches[portGroup.Config.DistributedVirtualSwitch.Value]
		if switchInfo == nil {
			continue
		}

		info := manager.PortGroupInfo{
			Name:    portGroup.Name,
			VLAN:    distributedPortGroupVLAN(portGroup),
			VMCount: len(portGroup.Vm),
		}
		for _, hostRef := range portGroup.Host {
			if hostName, ok := hostNames[hostRef.Value]; ok {
				info.Hosts = manager.AppendUnique(info.Hosts, hostName)
				switchInfo.Hosts = manager.AppendUnique(switchInfo.Hosts, hostName)
			}
		}
		switchInfo.PortGroups = append(switchInfo.PortGroups, info)
	}
}

func distributedPortGroupVLAN(portGroup mo.DistributedVirtualPortgroup) string {
	if cfg, ok := portGroup.Config.DefaultPortConfig.(*types.VMwareDVSPortSetting); ok && cfg.Vlan != nil {
		switch vlan := cfg.Vlan.(type) {
		case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
			return manager.FormatVLAN(vlan.VlanId)
		case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
			return "trunk"
		case *types.VmwareDistributedVirtualSwitchPvlanSpec:
			return fmt.Sprintf("PVLAN %d", vlan.PvlanId)
		}
	}
	return "—"
}

func assembleNetworkSwitches(standardSwitches map[string]*manager.SwitchInfo, standardPortGroups map[string]map[string]*manager.PortGroupInfo, distributedSwitches map[string]*manager.SwitchInfo) []manager.SwitchInfo {
	switches := make([]manager.SwitchInfo, 0, len(standardSwitches)+len(distributedSwitches))

	for _, switchInfo := range standardSwitches {
		switchCopy := *switchInfo
		switchCopy.PortGroups = appendSortedStandardPortGroups(standardPortGroups[switchInfo.Name], switchCopy.PortGroups)
		sortSwitchInfo(&switchCopy)
		switches = append(switches, switchCopy)
	}

	for _, switchInfo := range distributedSwitches {
		switchCopy := *switchInfo
		switchCopy.PortGroups = append([]manager.PortGroupInfo(nil), switchInfo.PortGroups...)
		sortSwitchInfo(&switchCopy)
		switches = append(switches, switchCopy)
	}

	sort.Slice(switches, func(i, j int) bool {
		if switches[i].Type != switches[j].Type {
			return switches[i].Type < switches[j].Type
		}
		return switches[i].Name < switches[j].Name
	})

	return switches
}

func appendSortedStandardPortGroups(portGroups map[string]*manager.PortGroupInfo, dst []manager.PortGroupInfo) []manager.PortGroupInfo {
	if len(portGroups) == 0 {
		return dst
	}

	names := make([]string, 0, len(portGroups))
	for name := range portGroups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dst = append(dst, *portGroups[name])
	}
	return dst
}

func sortSwitchInfo(info *manager.SwitchInfo) {
	sort.Strings(info.Uplinks)
	sort.Strings(info.Hosts)
	for i := range info.PortGroups {
		sort.Strings(info.PortGroups[i].Hosts)
	}
	sort.Slice(info.PortGroups, func(i, j int) bool {
		return info.PortGroups[i].Name < info.PortGroups[j].Name
	})
}
