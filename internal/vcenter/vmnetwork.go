package vcenter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"xman/internal/jobs"
	"xman/internal/manager"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type vCenterDistributedPortgroupRef struct {
	Ref  types.ManagedObjectReference
	Name string
}

func hasDistributedVCenterAdapter(devices []types.BaseVirtualDevice) bool {
	for _, device := range devices {
		card, ok := device.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		ethernet := card.GetVirtualEthernetCard()
		if _, ok := ethernet.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
			return true
		}
	}
	return false
}

func vCenterNetworkOptionID(ref types.ManagedObjectReference) string {
	switch ref.Type {
	case "Network":
		return "network:" + ref.Value
	case "DistributedVirtualPortgroup":
		return "dvportgroup:" + ref.Value
	default:
		return ref.Type + ":" + ref.Value
	}
}

func parseVCenterNetworkOptionID(id string) (types.ManagedObjectReference, error) {
	prefix, value, ok := strings.Cut(strings.TrimSpace(id), ":")
	if !ok || strings.TrimSpace(value) == "" {
		return types.ManagedObjectReference{}, fmt.Errorf("invalid network identifier %q", id)
	}

	switch prefix {
	case "network":
		return types.ManagedObjectReference{Type: "Network", Value: value}, nil
	case "dvportgroup":
		return types.ManagedObjectReference{Type: "DistributedVirtualPortgroup", Value: value}, nil
	default:
		return types.ManagedObjectReference{}, fmt.Errorf("unsupported vCenter network identifier %q", id)
	}
}

func updatedVCenterConnectable(current *types.VirtualDeviceConnectInfo, connected bool) *types.VirtualDeviceConnectInfo {
	if current == nil {
		return &types.VirtualDeviceConnectInfo{
			Connected:      connected,
			StartConnected: connected,
		}
	}

	next := *current
	next.Connected = connected
	next.StartConnected = connected
	return &next
}

func (b *Backend) distributedPortgroupsByKey(ctx context.Context) (map[string]vCenterDistributedPortgroupRef, error) {
	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	viewManager := view.NewManager(client.Client)
	portGroupView, err := viewManager.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, fmt.Errorf("dvpg view: %w", err)
	}
	defer portGroupView.Destroy(ctx)

	var portGroups []mo.DistributedVirtualPortgroup
	if err := portGroupView.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "key"}, &portGroups); err != nil {
		return nil, fmt.Errorf("fetching distributed port groups: %w", err)
	}

	out := make(map[string]vCenterDistributedPortgroupRef, len(portGroups))
	for _, portGroup := range portGroups {
		out[portGroup.Key] = vCenterDistributedPortgroupRef{
			Ref:  portGroup.Reference(),
			Name: strings.TrimSpace(portGroup.Name),
		}
	}
	return out, nil
}

func (b *Backend) ListVMNetworkOptions(ctx context.Context, vmRef string) ([]manager.VMNetworkOption, error) {
	vm, err := b.vmObject(ctx, vmRef)
	if err != nil {
		return nil, err
	}
	if _, err := vm.ObjectName(ctx); err != nil {
		return nil, fmt.Errorf("resolving VM for network options: %w", err)
	}

	client, err := b.session.Client()
	if err != nil {
		return nil, err
	}

	viewManager := view.NewManager(client.Client)

	networkView, err := viewManager.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"Network"}, true)
	if err != nil {
		return nil, fmt.Errorf("network view: %w", err)
	}
	defer networkView.Destroy(ctx)

	var standardNetworks []mo.Network
	if err := networkView.Retrieve(ctx, []string{"Network"}, []string{"name"}, &standardNetworks); err != nil {
		return nil, fmt.Errorf("fetching standard networks: %w", err)
	}

	dvSwitches, err := listDistributedSwitches(ctx, viewManager, client.ServiceContent.RootFolder)
	if err != nil {
		return nil, err
	}
	switchNames := make(map[string]string, len(dvSwitches))
	for _, dvSwitch := range dvSwitches {
		switchNames[dvSwitch.Reference().Value] = strings.TrimSpace(dvSwitch.Name)
	}

	portGroups, err := listDistributedPortGroups(ctx, viewManager, client.ServiceContent.RootFolder)
	if err != nil {
		return nil, err
	}

	options := make([]manager.VMNetworkOption, 0, len(standardNetworks)+len(portGroups))
	for _, network := range standardNetworks {
		name := strings.TrimSpace(network.Name)
		if name == "" {
			name = network.Reference().Value
		}
		options = append(options, manager.VMNetworkOption{
			ID:   vCenterNetworkOptionID(network.Reference()),
			Name: name,
			Type: "Standard",
		})
	}

	for _, portGroup := range portGroups {
		name := strings.TrimSpace(portGroup.Name)
		if name == "" {
			name = portGroup.Reference().Value
		}

		group := ""
		if portGroup.Config.DistributedVirtualSwitch != nil {
			group = switchNames[portGroup.Config.DistributedVirtualSwitch.Value]
		}

		options = append(options, manager.VMNetworkOption{
			ID:    vCenterNetworkOptionID(portGroup.Reference()),
			Name:  name,
			Type:  "Distributed",
			Group: group,
		})
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].Type != options[j].Type {
			return options[i].Type < options[j].Type
		}
		if options[i].Group != options[j].Group {
			return options[i].Group < options[j].Group
		}
		return options[i].Name < options[j].Name
	})

	return options, nil
}

func (b *Backend) UpdateVMNetwork(ctx context.Context, emit jobs.EmitFn, req manager.VMNetworkUpdateRequest) error {
	emit(10, "Loading current VM network settings...")
	info, err := b.GetVM(ctx, req.VMRef)
	if err != nil {
		return err
	}
	if info.PowerState != "poweredOff" {
		return fmt.Errorf("VM network changes require the VM to be powered off")
	}

	var currentAdapter *manager.VMNetworkAdapter
	for i := range info.NetworkAdapters {
		if info.NetworkAdapters[i].ID == req.AdapterID {
			currentAdapter = &info.NetworkAdapters[i]
			break
		}
	}
	if currentAdapter == nil {
		return fmt.Errorf("network adapter %q not found", req.AdapterID)
	}
	if currentAdapter.NetworkID == req.NetworkID && currentAdapter.Connected == req.Connected {
		emit(100, "Network attachment already matches the requested values.")
		return nil
	}

	client, err := b.session.Client()
	if err != nil {
		return err
	}

	vm, err := b.vmObject(ctx, req.VMRef)
	if err != nil {
		return err
	}

	targetRef, err := parseVCenterNetworkOptionID(req.NetworkID)
	if err != nil {
		return err
	}

	var backing types.BaseVirtualDeviceBackingInfo
	switch targetRef.Type {
	case "Network":
		backing, err = object.NewNetwork(client.Client, targetRef).EthernetCardBackingInfo(ctx)
	case "DistributedVirtualPortgroup":
		backing, err = object.NewDistributedVirtualPortgroup(client.Client, targetRef).EthernetCardBackingInfo(ctx)
	default:
		err = fmt.Errorf("unsupported vCenter network type %q", targetRef.Type)
	}
	if err != nil {
		return fmt.Errorf("resolving network backing: %w", err)
	}

	var obj mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &obj); err != nil {
		return fmt.Errorf("reading VM devices: %w", err)
	}

	var targetDevice types.BaseVirtualDevice
	for _, device := range obj.Config.Hardware.Device {
		card, ok := device.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}

		ethernet := card.GetVirtualEthernetCard()
		if fmt.Sprint(ethernet.Key) != req.AdapterID {
			continue
		}

		ethernet.Backing = backing
		ethernet.Connectable = updatedVCenterConnectable(ethernet.Connectable, req.Connected)
		targetDevice = device
		break
	}
	if targetDevice == nil {
		return fmt.Errorf("network adapter %q not found", req.AdapterID)
	}

	emit(60, "Applying network attachment...")
	task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		DeviceChange: []types.BaseVirtualDeviceConfigSpec{
			&types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    targetDevice,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("reconfiguring VM network: %w", err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for VM network reconfigure: %w", err)
	}

	emit(100, "Network attachment updated.")
	return nil
}
