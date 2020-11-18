package server

import (
	"fmt"
	"sync/atomic"

	"github.com/UpCloudLtd/upcloud-go-api/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/service"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/UpCloudLtd/cli/internal/commands"
	"github.com/UpCloudLtd/cli/internal/ui"
)

func StopCommand(service service.Server) commands.Command {
	return &stopCommand{
		BaseCommand: commands.New("stop", "Stop a server"),
		service:     service,
	}
}

type stopCommand struct {
	*commands.BaseCommand
	service  service.Server
	stopType string
}

func (s *stopCommand) InitCommand() {
	s.ArgCompletion(func(toComplete string) ([]string, cobra.ShellCompDirective) {
		servers, err := s.service.GetServers()
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		var vals []string
		for _, v := range servers.Servers {
			vals = append(vals, v.UUID, v.Hostname)
		}
		return commands.MatchStringPrefix(vals, toComplete, false), cobra.ShellCompDirectiveNoFileComp
	})
	flags := &pflag.FlagSet{}
	flags.StringVar(&s.stopType, "type", upcloud.StopTypeSoft,
		"The type of stop operation. Soft waits for the OS to shut down cleanly "+
			"while hard forcibly shuts down a server")
	s.AddFlags(flags)
	s.SetPositionalArgHelp("<uuidHostnameOrTitle ...>")
}

func (s *stopCommand) MakeExecuteCommand() func(args []string) (interface{}, error) {
	return func(args []string) (interface{}, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("server hostname, title or uuid is required")
		}
		var (
			allServers  []upcloud.Server
			stopServers []*upcloud.Server
		)
		for _, v := range args {
			server, err := searchServer(&allServers, s.service, v, true)
			if err != nil {
				return nil, err
			}
			stopServers = append(stopServers, server)
		}
		var numOk int64
		handler := func(idx int, e *ui.LogEntry) {
			server := stopServers[idx]
			msg := fmt.Sprintf("Stopping %q", server.Title)
			e.SetMessage(msg)
			e.Start()
			_, err := s.service.StopServer(&request.StopServerRequest{
				UUID:     server.UUID,
				Timeout:  s.Config().ClientTimeout(),
				StopType: s.stopType,
			})
			if err == nil {
				e.SetMessage(fmt.Sprintf("%s: shutdown request sent", msg))
				_, err = WaitForServerState(s.service, server.UUID, upcloud.ServerStateStopped, s.Config().ClientTimeout())
			}
			if err != nil {
				e.SetMessage(ui.LiveLogEntryErrorColours.Sprintf("%s: failed", msg))
				e.SetDetails(err.Error(), "error: ")
			} else {
				atomic.AddInt64(&numOk, 1)
				e.SetMessage(fmt.Sprintf("%s: done", msg))
			}
		}
		ui.StartWorkQueue(ui.WorkQueueConfig{
			NumTasks:           len(stopServers),
			MaxConcurrentTasks: maxServerActions,
			EnableUI:           s.Config().InteractiveUI(),
		}, handler)

		if int(numOk) < len(stopServers) {
			return nil, fmt.Errorf("number of servers failed to shut down: %d", len(stopServers)-int(numOk))
		}
		return stopServers, nil
	}
}
