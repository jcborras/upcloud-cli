package server

import (
	"bufio"
	"fmt"
	"github.com/UpCloudLtd/cli/internal/commands/storage"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/service"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"os"
	"strings"

	"github.com/UpCloudLtd/cli/internal/commands"
	"github.com/UpCloudLtd/cli/internal/ui"
)

func CreateCommand(serverSvc service.Server, storageSvc service.Storage) commands.Command {
	return &createCommand{
		BaseCommand: commands.New("create", "Create a server"),
		serverSvc:   serverSvc,
		storageSvc:  storageSvc,
	}
}

var DefaultCreateParams = &createParams{
	CreateServerRequest: request.CreateServerRequest{
		VideoModel: "vga",
		TimeZone:   "UTC",
		Plan:       "1xCPU-2GB",
	},
	firewall:       false,
	metadata:       false,
	os:             "Debian GNU/Linux 10 (Buster)",
	osStorageSize:  0,
	sshKeys:        nil,
	username:       "",
	createPassword: true,
}

type createParams struct {
	request.CreateServerRequest
	firewall      bool
	metadata      bool
	os            string
	osStorageSize int

	storages []string
	networks []string

	sshKeys        []string
	username       string
	createPassword bool
	remoteAccess   bool
}

func (s *createParams) processParams(storageSvc service.Storage) error {
	if s.os != "" {
		var osStorage *upcloud.Storage

		osStorage, err := storage.SearchSingleStorage(s.os, storageSvc)
		if err != nil {
			return err
		}

		size := minStorageSize
		if s.osStorageSize > size {
			size = s.osStorageSize
		}
		s.StorageDevices = append(s.StorageDevices, request.CreateServerStorageDevice{
			Action:  "clone",
			Storage: osStorage.UUID,
			Title:   fmt.Sprintf("%s-osDisk", ui.TruncateText(s.Hostname, 64-7)),
			Size:    size,
			Tier:    upcloud.StorageTierMaxIOPS,
			Type:    upcloud.StorageTypeDisk,
		})
	}

	if s.osStorageSize != 0 {
		s.StorageDevices[0].Size = s.osStorageSize
	}

	if s.firewall {
		s.Firewall = "on"
	}
	if s.metadata {
		s.Metadata = 1
	}
	if s.LoginUser == nil {
		s.LoginUser = &request.LoginUser{}
	}
	s.LoginUser.CreatePassword = "no"
	if s.createPassword {
		s.LoginUser.CreatePassword = "yes"
	}
	if s.username != "" {
		s.LoginUser.Username = s.username
	}
	if s.remoteAccess {
		s.RemoteAccessEnabled = upcloud.FromBool(true)
	}

	return nil
}

func (s *createParams) handleStorage(in string, storageSvc service.Storage) (*request.CreateServerStorageDevice, error) {
	sd := &request.CreateServerStorageDevice{}
	fs := &pflag.FlagSet{}
	args, err := commands.Parse(in)
	if err != nil {
		return nil, err
	}
	fs.StringVar(&sd.Action, "action", sd.Action, "")
	fs.StringVar(&sd.Address, "address", sd.Address, "")
	fs.StringVar(&sd.Storage, "storage", sd.Storage, "")
	fs.StringVar(&sd.Type, "type", sd.Type, "")
	fs.StringVar(&sd.Tier, "tier", sd.Tier, "")
	fs.StringVar(&sd.Title, "title", sd.Title, "")
	fs.IntVar(&sd.Size, "size", sd.Size, "")
	err = fs.Parse(args)
	if err != nil {
		return nil, err
	}

	if sd.Action != request.CreateServerStorageDeviceActionCreate {
		if sd.Storage == "" {
			return nil, fmt.Errorf("storage UUID or Title must be provided for %s operation", sd.Action)
		}
		strg, err := storage.SearchSingleStorage(sd.Storage, storageSvc)
		if err != nil {
			return nil, err
		}
		sd.Storage = strg.UUID
	}

	if sd.Action == request.CreateServerStorageDeviceActionClone && sd.Title == "" {
		sd.Title = fmt.Sprintf("%s-%s-clone", ui.TruncateText(s.Hostname, 64-7-len(sd.Storage)), sd.Storage)
	}

	if sd.Action == request.CreateServerStorageDeviceActionCreate && sd.Title == "" {
		return nil, fmt.Errorf("title of new storage must be provided")
	}

	return sd, nil
}

func (s *createParams) handleNetwork(in string) (*request.CreateServerInterface, error) {
	network := &request.CreateServerInterface{
		Type: "private",
	}
	var family string
	fs := &pflag.FlagSet{}
	args, err := commands.Parse(in)
	if err != nil {
		return nil, err
	}
	fs.StringVar(&family, "family", family, "")
	fs.StringVar(&network.Type, "type", network.Type, "")
	err = fs.Parse(args)
	if err != nil {
		return nil, err
	}

	var ipAddresses []request.CreateServerIPAddress
	ipAddresses = append(ipAddresses, request.CreateServerIPAddress{Family: family})
	network.IPAddresses = ipAddresses

	return network, nil
}

func (s *createParams) handleSshKey() error {
	var allSshKeys []string
	for _, keyOrFile := range s.sshKeys {
		if strings.HasPrefix(keyOrFile, "ssh-") {
			if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyOrFile)); err != nil {
				return fmt.Errorf("invalid ssh key %q: %v", keyOrFile, err)
			}
			allSshKeys = append(allSshKeys, keyOrFile)
			continue
		}
		f, err := os.Open(keyOrFile)
		if err != nil {
			return err
		}
		rdr := bufio.NewScanner(f)
		for rdr.Scan() {
			if _, _, _, _, err := ssh.ParseAuthorizedKey(rdr.Bytes()); err != nil {
				_ = f.Close()
				return fmt.Errorf("invalid ssh key %q in file %s: %v", rdr.Text(), keyOrFile, err)
			}
			allSshKeys = append(allSshKeys, rdr.Text())
		}
		_ = f.Close()
	}
	s.LoginUser.SSHKeys = allSshKeys
	return nil
}

type createCommand struct {
	*commands.BaseCommand
	serverSvc  service.Server
	storageSvc service.Storage
	params     createParams
}

func (s *createCommand) InitCommand() {
	fs := &pflag.FlagSet{}
	s.params = createParams{CreateServerRequest: request.CreateServerRequest{}}
	def := DefaultCreateParams
	fs.IntVar(&s.params.AvoidHost, "avoid-host", def.AvoidHost, "Use this to make sure VMs do not reside on specific host. Refers to value from host -attribute. Useful when building HA-environments.")
	fs.IntVar(&s.params.Host, "host", def.Host, "Use this to start a VM on a specific host. Refers to value from host -attribute. Only available for private cloud hosts.")
	fs.StringVar(&s.params.BootOrder, "boot-order", def.BootOrder, "The boot device order, disk / cdrom / network or comma separated combination.")
	fs.StringVar(&s.params.UserData, "user-data", def.UserData, "Defines URL for a server setup script, or the script body itself.")
	fs.IntVar(&s.params.CoreNumber, "cores", def.CoreNumber, "Number of cores.")
	fs.IntVar(&s.params.MemoryAmount, "memory", def.MemoryAmount, "Memory amount in MiB.")
	fs.StringVar(&s.params.Title, "title", def.Title, "Visible name.")
	fs.StringVar(&s.params.Hostname, "hostname", def.Hostname, "Hostname.")
	fs.StringVar(&s.params.Plan, "plan", def.Plan, "Server plan to use. Set this to custom to use custom core/memory amounts.")
	fs.StringVar(&s.params.os, "os", def.os, "Server OS to use (will be the first storage device). Set to empty to fully customise the storages.")
	fs.IntVar(&s.params.osStorageSize, "os-storage-size", def.osStorageSize, "OS storage size in GiB. This is only applicable if `os` is also set. Zero value makes the disk equal to the minimum size of the template.")
	fs.StringVar(&s.params.Zone, "zone", def.Zone, "Zone where to create the server")
	fs.StringVar(&s.params.PasswordDelivery, "password-delivery", def.PasswordDelivery, "If password login is enable set a way how password is delivered.\nAvailable: email,sms")
	fs.StringVar(&s.params.SimpleBackup, "simple-backup", def.SimpleBackup, "Simple backup rule. Format (HHMM,{dailies,weeklies,monthlies}).\nExample: 2300,dailies")
	fs.StringVar(&s.params.TimeZone, "time-zone", def.TimeZone, "Time zone to set the RTC to")
	fs.StringVar(&s.params.VideoModel, "video-model", def.VideoModel, "Video interface model of the server.\nAvailable: vga,cirrus")
	fs.BoolVar(&s.params.firewall, "firewall", def.firewall, "Enables the firewall. You can manage firewall rules with the firewall command.")
	fs.BoolVar(&s.params.metadata, "metadata", def.metadata, "Enable metadata service.")
	fs.StringArrayVar(&s.params.storages, "storage", def.storages, "A storage connected to the server, multiple can be declared. \nUsage: --storage action=attach,storage=01000000-0000-4000-8000-000020010301,type=cdrom")
	fs.StringArrayVar(&s.params.networks, "network", def.networks, "A network interface for the server, multiple can be declared.\nUsage: --network family=IPv4,type=public")
	fs.BoolVar(&s.params.createPassword, "create-password", def.createPassword, "Create a admin password.")
	fs.StringVar(&s.params.username, "username", def.username, "Admin account username.")
	fs.StringSliceVar(&s.params.sshKeys, "ssh-keys", def.sshKeys, "Add one or more SSH keys to the admin account. Accepted values are SSH public keys or filenames from where to read the keys.")
	fs.BoolVar(&s.params.remoteAccess, "remote-access-enabled", def.remoteAccess, "Enables or disables the remote access.")
	fs.StringVar(&s.params.RemoteAccessType, "remote-access-type", def.RemoteAccessType, "The remote access type.")
	fs.StringVar(&s.params.RemoteAccessPassword, "remote-access-password", def.RemoteAccessPassword, "The remote access password.")
	s.AddFlags(fs)
}

func (s *createCommand) MakeExecuteCommand() func(args []string) (interface{}, error) {
	return func(args []string) (interface{}, error) {

		if s.params.Hostname == "" || s.params.Zone == "" || s.params.Title == "" {
			return nil, fmt.Errorf("hostname, title and zone are required")
		}

		if err := s.params.processParams(s.storageSvc); err != nil {
			return nil, err
		}

		req := s.params.CreateServerRequest

		var iFaces []request.CreateServerInterface
		for _, network := range s.params.networks {
			_interface, err := s.params.handleNetwork(network)
			if err != nil {
				return nil, err
			}
			iFaces = append(iFaces, *_interface)
		}

		for _, strg := range s.params.storages {
			strg, err := s.params.handleStorage(strg, s.storageSvc)
			if err != nil {
				return nil, err
			}
			req.StorageDevices = append(req.StorageDevices, *strg)
		}

		if err := s.params.handleSshKey(); err != nil {
			return nil, err
		}

		if len(iFaces) > 0 {
			req.Networking = &request.CreateServerNetworking{Interfaces: iFaces}
		}

		return ui.HandleContext{
			RequestID:     func(in interface{}) string { return in.(*request.CreateServerRequest).Hostname },
			ResultUUID:    getServerDetailsUuid,
			InteractiveUI: s.Config().InteractiveUI(),
			WaitMsg:       "server starting",
			WaitFn:        WaitForServerFn(s.serverSvc, upcloud.ServerStateStarted, s.Config().ClientTimeout()),
			MaxActions:    5,
			ActionMsg:     "Creating server",
			Action: func(req interface{}) (interface{}, error) {
				return s.serverSvc.CreateServer(req.(*request.CreateServerRequest))
			},
		}.Handle(commands.ToArray(&req))
	}
}
