package main

import (
	"log"

	"github.com/rombintu/cephapi/core"
)

func main() {
	config, err := core.NewConfig("./configs/default.toml")
	if err != nil {
		log.Fatal(err)
	}
	for name, cluster := range config.Clusters {
		c, err := core.NewCluster(name, config.Default.CredsPath, cluster)
		if err != nil {
			log.Fatal(err)
		}

		for _, pool := range c.Pools {
			// poolData := c.GetZoneByPoolName(pool.Name)
			c.Log.Debugf("%+v", pool)
		}

		c.Close()
	}
}
