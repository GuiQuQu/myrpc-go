package xclient

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

// 选择策略
const (
	RandomSelect     SelectMode = iota // select randomly
	RoundRobinSelect                   // select using Robbin algorithm
)

type Discovery interface {
	Refresh() error                      // refresh from remote registry
	Update(servers []string) error       // update with new servers
	Get(mode SelectMode) (string, error) // get a server according to mode
	GetAll() ([]string, error)           // get all servers in the registry
}

type MultiServersDiscovery struct {
	r       *rand.Rand // generate random number
	mu      sync.Mutex // protect following
	servers []string   // server addresses
	index   int        // record the selected position for robin algorithm
}

func NewMultiServerDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	if len(servers) > 0 {
		d.index = d.r.Intn(len(servers))
	} else {
		d.index = 0
	}
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil)

func (d *MultiServersDiscovery) Refresh() error {
	return nil
}

func (d *MultiServersDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}
