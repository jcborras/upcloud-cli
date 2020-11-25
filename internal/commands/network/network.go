package network

import (
	"fmt"
	"github.com/UpCloudLtd/cli/internal/commands"
	"github.com/UpCloudLtd/cli/internal/ui"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/service"
	"github.com/spf13/cobra"
)

const maxNetworkActions = 10
const positionalArgHelp = "<UUID/Name...>"

func NetworkCommand() commands.Command {
	return &networkCommand{commands.New("network", "Manage network")}
}

type networkCommand struct {
	*commands.BaseCommand
}

var getNetworkUuid = func(in interface{}) string { return in.(*upcloud.Network).UUID }

func SearchNetwork(uuidOrName string, service service.Network) (*upcloud.Network, error) {
	var result []upcloud.Network
	networks, err := service.GetNetworks()
	if err != nil {
		return nil, err
	}
	for _, network := range networks.Networks {
		if network.UUID == uuidOrName || network.Name == uuidOrName {
			result = append(result, network)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no network was found with %s", uuidOrName)
	}
	if len(result) > 1 {
		return nil, fmt.Errorf("multiple networks matched to query %q", uuidOrName)
	}
	return &result[0], nil
}

func searchNetworks(uuidOrNames []string, service service.Network) ([]*upcloud.Network, error) {
	var result []*upcloud.Network
	for _, uuidOrName := range uuidOrNames {
		ip, err := SearchNetwork(uuidOrName, service)
		if err != nil {
			return nil, err
		}
		result = append(result, ip)
	}
	return result, nil
}

type Request struct {
	ExactlyOne   bool
	BuildRequest func(storage *upcloud.Network) interface{}
	Service      service.Network
	ui.HandleContext
}

func (s Request) Send(args []string) (interface{}, error) {
	if s.ExactlyOne && len(args) != 1 {
		return nil, fmt.Errorf("single network uuid or name is required")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one network uuid or name is required")
	}

	servers, err := searchNetworks(args, s.Service)
	if err != nil {
		return nil, err
	}

	var requests []interface{}
	for _, server := range servers {
		requests = append(requests, s.BuildRequest(server))
	}

	return s.Handle(requests)
}

func GetArgCompFn(s service.Network) func(toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(toComplete string) ([]string, cobra.ShellCompDirective) {
		networks, err := s.GetNetworks()
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		var vals []string
		for _, v := range networks.Networks {
			vals = append(vals, v.UUID, v.Name)
		}
		return commands.MatchStringPrefix(vals, toComplete, true), cobra.ShellCompDirectiveNoFileComp
	}
}
