package nodes

import (
	"reflect"
	"testing"

	"github.com/celestiaorg/torch/config"
)

func TestValidateNode(t *testing.T) {
	type args struct {
		n   string
		cfg config.MutualPeersConfig
	}

	cfg := config.MutualPeersConfig{
		MutualPeers: []*config.MutualPeer{
			{
				Peers: []config.Peer{
					{NodeName: "da-bridge-1"},
					{NodeName: "da-bridge-2"},
				},
			},
			{
				Peers: []config.Peer{
					{NodeName: "da-bridge-3"},
				},
			},
		},
	}

	tests := []struct {
		name  string
		args  args
		want  bool
		want1 config.Peer
	}{
		{
			name: "Case 1: Node exists in config",
			args: args{
				n:   "da-bridge-1",
				cfg: cfg,
			},
			want:  true,
			want1: config.Peer{NodeName: "da-bridge-1"},
		},
		{
			name: "Case 2: Node exists in config",
			args: args{
				n:   "da-bridge-2",
				cfg: cfg,
			},
			want:  true,
			want1: config.Peer{NodeName: "da-bridge-2"},
		},
		{
			name: "Case 1: Node DOES NOT exists in config",
			args: args{
				n:   "nonexistent_node",
				cfg: cfg,
			},
			want:  false,
			want1: config.Peer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidateNode(tt.args.n, tt.args.cfg)
			if got != tt.want {
				t.Errorf("ValidateNode() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ValidateNode() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
