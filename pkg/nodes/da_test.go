package nodes

import (
	"reflect"
	"testing"

	"github.com/celestiaorg/torch/config"
)

func TestHasAddrAlready(t *testing.T) {
	t.Parallel()
	type args struct {
		peer      config.Peer
		i         int
		c         string
		addPrefix bool
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name: "Case 1.0: DNS - Multi address specified",
			args: args{
				peer: config.Peer{
					NodeName:   "da-full-1",
					NodeType:   "da",
					ConnectsTo: []string{"/dns/da-bridge-1/tcp/2121/p2p/12D3KooWH1pTTJR5NXPYs2huVcJ9srmmiyGU4txHm2qgdaUVPYAw"},
				},
				i:         0,
				c:         "",
				addPrefix: false,
			},
			want:  "/dns/da-bridge-1/tcp/2121/p2p/12D3KooWH1pTTJR5NXPYs2huVcJ9srmmiyGU4txHm2qgdaUVPYAw",
			want1: false,
		},
		{
			name: "Case 1.1: IP - Multi address specified",
			args: args{
				peer: config.Peer{
					NodeName:   "da-full-1",
					NodeType:   "da",
					ConnectsTo: []string{"/ip4/192.168.1.100/tcp/2121/p2p/12D3KooWH1pTTJR5NXPYs2huVcJ9srmmiyGU4txHm2qgdaUVPYAw"},
				},
				i:         0,
				c:         "",
				addPrefix: false,
			},
			want:  "/ip4/192.168.1.100/tcp/2121/p2p/12D3KooWH1pTTJR5NXPYs2huVcJ9srmmiyGU4txHm2qgdaUVPYAw",
			want1: false,
		},
		{
			name: "Case 2: No multi address specified - one node",
			args: args{
				peer: config.Peer{
					NodeName:   "da-full-1",
					NodeType:   "da",
					ConnectsTo: []string{"da-bridge-1"},
				},
				i:         0,
				c:         "da-bridge-1",
				addPrefix: false,
			},
			want:  "da-bridge-1",
			want1: false,
		},
		{
			name: "Case 3: No multi address specified - more than one node",
			args: args{
				peer: config.Peer{
					NodeName:   "da-full-1",
					NodeType:   "da",
					ConnectsTo: []string{"da-bridge-1", "da-bridge-2"},
				},
				i:         1,
				c:         "da-bridge-1,da-bridge-2",
				addPrefix: false,
			},
			want:  "da-bridge-1,da-bridge-2",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := HasAddrAlready(tt.args.peer, tt.args.i, tt.args.c, tt.args.addPrefix)
			if got != tt.want {
				t.Errorf("HasAddrAlready() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("HasAddrAlready() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestSetDaNodeDefault(t *testing.T) {
	type args struct {
		peer config.Peer
	}
	tests := []struct {
		name string
		args args
		want config.Peer
	}{
		{
			name: "Case 1: Tests default values",
			args: args{
				peer: config.Peer{
					NodeName: "da-full-1",
					NodeType: "da",
				},
			},
			want: config.Peer{
				NodeName:           "da-full-1",
				NodeType:           "da",
				ContainerName:      "da",
				ContainerSetupName: "da-setup",
				ConnectsAsEnvVar:   false,
				ConnectsTo:         nil,
				DnsConnections:     nil,
			},
		},
		{
			name: "Case 2: Tests default values already specified",
			args: args{
				peer: config.Peer{
					NodeName:           "da-bridge-1",
					NodeType:           "da",
					ContainerName:      "da",
					ContainerSetupName: "da-setup",
				},
			},
			want: config.Peer{
				NodeName:           "da-bridge-1",
				NodeType:           "da",
				ContainerName:      "da",
				ContainerSetupName: "da-setup",
				ConnectsAsEnvVar:   false,
				ConnectsTo:         nil,
				DnsConnections:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetDaNodeDefault(tt.args.peer); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetDaNodeDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
