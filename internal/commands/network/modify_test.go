package network

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-cli/internal/commands"
	"github.com/UpCloudLtd/upcloud-cli/internal/config"
	smock "github.com/UpCloudLtd/upcloud-cli/internal/mock"
	"github.com/UpCloudLtd/upcloud-cli/internal/output"

	"github.com/UpCloudLtd/upcloud-go-api/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/upcloud/request"
	"github.com/gemalto/flume"
	"github.com/stretchr/testify/assert"
)

func TestModifyCommand(t *testing.T) {
	t.Parallel()
	n := upcloud.Network{
		UUID:   "9abccbe8-8d47-40dd-a5af-c6598f38b11b",
		Name:   "test-network",
		Zone:   "fi-hel1",
		Router: "",
	}

	for _, test := range []struct {
		name     string
		flags    []string
		error    string
		expected request.ModifyNetworkRequest
	}{
		{
			name: "family is missing",
			flags: []string{
				"--name", n.Name,
				"--ip-network", "gateway=gw,dhcp=true",
			},
			error: "family is required",
		},
		{
			name: "with single network",
			flags: []string{
				"--name", n.Name,
				"--ip-network", "family=IPv4,\"dhcp-dns=one,two,three\",gateway=gw,dhcp=true",
			},
			expected: request.ModifyNetworkRequest{
				UUID: n.UUID,
				Name: n.Name,
				IPNetworks: []upcloud.IPNetwork{
					{
						Family:  upcloud.IPAddressFamilyIPv4,
						DHCP:    upcloud.FromBool(true),
						DHCPDns: []string{"one", "two", "three"},
						Gateway: "gw",
					},
				},
			},
		},
		{
			name: "with multiple network",
			flags: []string{
				"--name", n.Name,
				"--ip-network", "\"dhcp-dns=one,two,three\",gateway=gw,dhcp=false,family=IPv4", "--ip-network", "family=IPv6,dhcp-dns=four",
			},
			expected: request.ModifyNetworkRequest{
				UUID: n.UUID,
				Name: n.Name,
				IPNetworks: []upcloud.IPNetwork{
					{
						Family:  upcloud.IPAddressFamilyIPv4,
						DHCP:    upcloud.FromBool(false),
						DHCPDns: []string{"one", "two", "three"},
						Gateway: "gw",
					},
					{
						Family:  upcloud.IPAddressFamilyIPv6,
						DHCPDns: []string{"four"},
					},
				},
			},
		},
	} {
		// grab a local reference for parallel tests
		test := test
		targetMethod := "ModifyNetwork"
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mService := smock.Service{}
			mService.On(targetMethod, &test.expected).Return(&upcloud.Network{}, nil)
			mService.On("GetNetworks").Return(&upcloud.Networks{Networks: []upcloud.Network{n}}, nil)
			conf := config.New()
			c := commands.BuildCommand(ModifyCommand(), nil, conf)
			err := c.Cobra().Flags().Parse(test.flags)
			assert.NoError(t, err)

			_, err = c.(commands.SingleArgumentCommand).ExecuteSingleArgument(
				commands.NewExecutor(conf, &mService, flume.New("test")),
				n.UUID,
			)

			if err != nil {
				assert.EqualError(t, err, test.error)
			} else {
				mService.AssertNumberOfCalls(t, targetMethod, 1)
			}
		})
	}
}

func TestModifyCommandAttach(t *testing.T) {
	t.Parallel()
	n := upcloud.Network{
		UUID:   "9abccbe8-8d47-40dd-a5af-c6598f38b11b",
		Name:   "test-network",
		Zone:   "fi-hel1",
		Router: "",
	}
	r := upcloud.Router{
		AttachedNetworks: nil,
		Name:             "test-router",
		UUID:             "fakeuuid",
	}
	for _, test := range []struct {
		name     string
		flags    []string
		error    string
		expected request.AttachNetworkRouterRequest
	}{
		{
			name: "attach router with uuid",
			flags: []string{
				"--router", "fakeuuid",
			},
			expected: request.AttachNetworkRouterRequest{
				NetworkUUID: n.UUID,
				RouterUUID:  "fakeuuid",
			},
		},
		{
			name: "attach router with name",
			flags: []string{
				"--router", "test-router",
			},
			expected: request.AttachNetworkRouterRequest{
				NetworkUUID: n.UUID,
				RouterUUID:  "fakeuuid",
			},
		},
	} {
		targetMethod := "AttachNetworkRouter"
		// grab a local reference for parallel tests
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testSimpleModifyCommand(t, test.flags, test.error, n, r, targetMethod, &test.expected)
		})
	}
}

func TestModifyCommandDetach(t *testing.T) {
	t.Parallel()
	n := upcloud.Network{
		UUID:   "9abccbe8-8d47-40dd-a5af-c6598f38b11b",
		Name:   "test-network",
		Zone:   "fi-hel1",
		Router: "",
	}
	r := upcloud.Router{
		AttachedNetworks: nil,
		Name:             "test-router",
		UUID:             "fakeuuid",
	}
	for _, test := range []struct {
		name     string
		flags    []string
		error    string
		expected request.DetachNetworkRouterRequest
	}{
		{
			name: "detach router",
			flags: []string{
				"--detach-router",
			},
			expected: request.DetachNetworkRouterRequest{
				NetworkUUID: n.UUID,
			},
		},
	} {
		targetMethod := "DetachNetworkRouter"
		// grab a local reference for parallel tests
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testSimpleModifyCommand(t, test.flags, test.error, n, r, targetMethod, &test.expected)
		})
	}
}

func testSimpleModifyCommand(t *testing.T, flags []string, expectedError string, network upcloud.Network, router upcloud.Router, calledMethod string, expectedRequest interface{}) {
	t.Helper()
	mService := smock.Service{}
	mService.On(calledMethod, expectedRequest).Return(nil)
	mService.On("GetNetworkDetails", &request.GetNetworkDetailsRequest{UUID: network.UUID}).Return(&network, nil)
	mService.On("GetNetworks").Return(&upcloud.Networks{Networks: []upcloud.Network{network}}, nil)
	mService.On("GetRouters").Return(&upcloud.Routers{Routers: []upcloud.Router{router}}, nil)
	conf := config.New()
	c := commands.BuildCommand(ModifyCommand(), nil, conf)
	err := c.Cobra().Flags().Parse(flags)
	assert.NoError(t, err)

	_, err = c.(commands.SingleArgumentCommand).ExecuteSingleArgument(
		commands.NewExecutor(conf, &mService, flume.New("test")),
		network.UUID,
	)

	if err != nil {
		assert.EqualError(t, err, expectedError)
	} else {
		assert.Equal(t, "", expectedError, "expected error but received none")
		mService.AssertNumberOfCalls(t, calledMethod, 1)
	}
}

func TestModifyCommandModifyAndAttach(t *testing.T) {
	t.Parallel()
	n := upcloud.Network{
		UUID:   "9abccbe8-8d47-40dd-a5af-c6598f38b11b",
		Name:   "test-network",
		Zone:   "fi-hel1",
		Router: "",
	}
	r := upcloud.Router{
		AttachedNetworks: nil,
		Name:             "test-router",
		UUID:             "fakeuuid",
	}
	for _, test := range []struct {
		name           string
		flags          []string
		error          string
		expectedModify request.ModifyNetworkRequest
		expectedAttach request.AttachNetworkRouterRequest
	}{
		{
			name: "change name and attach router with uuid",
			flags: []string{
				"--name", "foo",
				"--router", "fakeuuid",
			},
			expectedModify: request.ModifyNetworkRequest{
				UUID: n.UUID,
				Name: "foo",
			},
			expectedAttach: request.AttachNetworkRouterRequest{
				NetworkUUID: n.UUID,
				RouterUUID:  "fakeuuid",
			},
		},
		{
			name: "change name and attach router with name",
			flags: []string{
				"--name", "foo",
				"--router", "test-router",
			},
			expectedModify: request.ModifyNetworkRequest{
				UUID: n.UUID,
				Name: "foo",
			},
			expectedAttach: request.AttachNetworkRouterRequest{
				NetworkUUID: n.UUID,
				RouterUUID:  "fakeuuid",
			},
		},
	} {
		// grab a local reference for parallel tests
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mService := smock.Service{}
			mService.On("AttachNetworkRouter", &test.expectedAttach).Return(nil)
			mService.On("ModifyNetwork", &test.expectedModify).Return(&n, nil)
			mService.On("GetNetworkDetails", &request.GetNetworkDetailsRequest{UUID: n.UUID}).Return(&n, nil)
			mService.On("GetNetworks").Return(&upcloud.Networks{Networks: []upcloud.Network{n}}, nil)
			mService.On("GetRouters").Return(&upcloud.Routers{Routers: []upcloud.Router{r}}, nil)
			conf := config.New()
			c := commands.BuildCommand(ModifyCommand(), nil, conf)
			err := c.Cobra().Flags().Parse(test.flags)
			assert.NoError(t, err)

			result, err := c.(commands.SingleArgumentCommand).ExecuteSingleArgument(
				commands.NewExecutor(conf, &mService, flume.New("test")),
				n.UUID,
			)
			if err != nil {
				assert.EqualError(t, err, test.error)
			} else {
				mService.AssertNumberOfCalls(t, "AttachNetworkRouter", 1)
				mService.AssertNumberOfCalls(t, "ModifyNetwork", 1)
				mService.AssertNumberOfCalls(t, "GetRouters", 1)
				// validate the edge case here which should not call GetNetworkDetails (as the modify returns the latest state)
				// but still updates the router manually
				mService.AssertNumberOfCalls(t, "GetNetworkDetails", 0)
				assert.Equal(t, r.UUID, result.(output.OnlyMarshaled).Value.(*upcloud.Network).Router)
			}
		})
	}
}

func TestModifyCommandModifyAndDetach(t *testing.T) {
	t.Parallel()
	n := upcloud.Network{
		UUID:   "9abccbe8-8d47-40dd-a5af-c6598f38b11b",
		Name:   "test-network",
		Zone:   "fi-hel1",
		Router: "fakeuuid",
	}
	for _, test := range []struct {
		name           string
		flags          []string
		error          string
		expectedModify request.ModifyNetworkRequest
		expectedDetach request.DetachNetworkRouterRequest
	}{
		{
			name: "change name and detach router",
			flags: []string{
				"--name", "foo",
				"--detach-router",
			},
			expectedModify: request.ModifyNetworkRequest{
				UUID: n.UUID,
				Name: "foo",
			},
			expectedDetach: request.DetachNetworkRouterRequest{
				NetworkUUID: n.UUID,
			},
		},
	} {
		// grab a local reference for parallel tests
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mService := smock.Service{}
			mService.On("DetachNetworkRouter", &test.expectedDetach).Return(nil)
			mService.On("ModifyNetwork", &test.expectedModify).Return(&n, nil)
			mService.On("GetNetworkDetails", &request.GetNetworkDetailsRequest{UUID: n.UUID}).Return(&n, nil)
			mService.On("GetNetworks").Return(&upcloud.Networks{Networks: []upcloud.Network{n}}, nil)
			conf := config.New()
			c := commands.BuildCommand(ModifyCommand(), nil, conf)
			err := c.Cobra().Flags().Parse(test.flags)
			assert.NoError(t, err)

			result, err := c.(commands.SingleArgumentCommand).ExecuteSingleArgument(
				commands.NewExecutor(conf, &mService, flume.New("test")),
				n.UUID,
			)
			if err != nil {
				assert.EqualError(t, err, test.error)
			} else {
				mService.AssertNumberOfCalls(t, "DetachNetworkRouter", 1)
				mService.AssertNumberOfCalls(t, "ModifyNetwork", 1)
				// validate the edge case here which should not call GetNetworkDetails (as the modify returns the latest state)
				// but still updates the router manually
				mService.AssertNumberOfCalls(t, "GetNetworkDetails", 0)
				assert.Equal(t, "", result.(output.OnlyMarshaled).Value.(*upcloud.Network).Router)
			}
		})
	}
}
