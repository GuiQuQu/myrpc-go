package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type MyRegistryDiscovery struct {
	*MultiServersDiscovery
	registry string

	// 服务列表过期时间
	timeout time.Duration
	// 注册中心最后一次更新服务列表时间
	lastUpdate time.Time
}

const defaultUpdateTimeout = time.Second * 10

func NewMyRegistryDiscovery(registryAddr string, timeout time.Duration) *MyRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	return &MyRegistryDiscovery{
		MultiServersDiscovery: NewMultiServerDiscovery(make([]string, 0)),
		registry:              registryAddr,
		timeout:               timeout,
	}
}

func (d *MyRegistryDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	d.lastUpdate = time.Now()
	return nil
}

func (d *MyRegistryDiscovery) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}

	log.Println("rpc registry: refresh servers from registry", d.registry)
	resp, err := http.Get(d.registry)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Myrpc-Servers"), ",")
	d.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			d.servers = append(d.servers, strings.TrimSpace(server))
		}
	}
	d.lastUpdate = time.Now()
	// log.Println("rpc registry: refresh servers result:", d.servers)
	return nil
}

func (d *MyRegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := d.Refresh(); err != nil {
		return "", err
	}
	return d.MultiServersDiscovery.Get(mode)
}

func (d *MyRegistryDiscovery) GetAll() ([]string, error) {
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	return d.MultiServersDiscovery.GetAll()
}
