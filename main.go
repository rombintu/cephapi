package main

import (
	"flag"
	"log"
	"sync"

	"github.com/rombintu/cephapi/core"
)

func worker(clusterName, confPath string, cluster core.ClusterConf) {
	c, err := core.NewCluster(clusterName, confPath, cluster)
	if err != nil {
		log.Fatal(err)
	}

	c.GetZonesStat()
	c.Calculate()
	for _, zone := range c.RootZones {
		c.Log.Debugf("Zone [%s] %+v", zone.PublicName, zone)
	}

	c.Close()
}

func main() {
	confFile := flag.String("conf", "./configs/default.toml", "Конфигурационный файл для cephapi")
	flag.Parse()
	config, err := core.NewConfig(*confFile)
	if err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	for name, clusterConf := range config.Clusters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(name, config.Default.CredsPath, clusterConf)
		}()
	}
	wg.Wait()
}
