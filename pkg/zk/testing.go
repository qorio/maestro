package zk

import (
	"github.com/golang/glog"
	"os"
	"strings"
)

func test_zkhosts() []string {
	servers := []string{"localhost:2181"}
	list := os.Getenv("ZK_HOSTS")
	if len(list) > 0 {
		servers = strings.Split(list, ",")
	}
	glog.Infoln("ZK_HOSTS = ", servers)
	return servers
}
