package core

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"

	"github.com/ceph/go-ceph/rados"
	"github.com/sirupsen/logrus"
)

const (
	PREFIX           string = "prefix"
	FORMAT           string = "format"
	POOL             string = "pool"
	VAR              string = "var"
	ARGS             string = "args"
	CRUSHROLE        string = "crush_rule"
	CRHUSHROLELEGASY string = "crush_ruleset"
)

type Pool struct {
	Name      string
	ZoneId    int
	ZoneName  string
	Used      Size
	Provision Size
}

type Cluster struct {
	ID           int
	Name         string
	Conn         *rados.Conn
	IOctx        *rados.IOContext
	Pools        []Pool
	Zones        []string
	Config       ClusterConf
	cephConfPath string
	Log          *logrus.Logger
	Version      int
}

func NewCluster(clusterName, credsPath string, conf ClusterConf) (*Cluster, error) {
	c := &Cluster{
		Name:         clusterName,
		Config:       conf,
		cephConfPath: path.Join(credsPath, conf.Conf),
		Log:          NewLogger(clusterName),
	}

	conn, err := rados.NewConn()
	if err != nil {
		return c, err
	}
	c.Conn = conn
	c.Log.Info("Use ceph.conf: ", c.cephConfPath)
	if err := c.Conn.ReadConfigFile(c.cephConfPath); err != nil {
		return c, err
	}
	if err := c.Open(); err != nil {
		return c, err
	}
	version, err := c.GetVersion()
	if err != nil {
		c.Log.Error(err)
		version = 12
	}
	c.Version = version
	pools, err := c.Conn.ListPools()
	if err != nil {
		return c, err
	}
	c.Zones = c.ZonesList()
	for _, pool := range pools {
		poolData := c.GetZoneByPoolName(pool)
		zoneId, ok := poolData["pool_id"].(float64)
		if !ok {
			c.Log.Error(err)
			zoneId = -1
		}
		zoneName, ok := poolData["crush_rule"].(string)
		if !ok {
			c.Log.Error(err)
			zoneName = "None"
		}
		used, provision := c.GetPoolStat(pool)
		c.Pools = append(c.Pools, Pool{
			Name: pool, Used: used,
			ZoneId:    int(zoneId),
			ZoneName:  zoneName,
			Provision: provision,
		})
	}
	// c.Zones = zones
	// c.Close()
	return c, nil
}

func (c *Cluster) Open() error {
	c.Log.Debug("Connect")
	if err := c.Conn.Connect(); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) Close() {
	c.Log.Debug("Disconnect")
	c.Conn.Shutdown()
}

func (c *Cluster) GetZoneByPoolName(pool string) (payload map[string]interface{}) {
	// var payload map[string]interface{}
	var crushrole string
	if c.Version <= 10 {
		crushrole = CRHUSHROLELEGASY
	} else {
		crushrole = CRUSHROLE
	}
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd pool get",
			POOL:   pool,
			VAR:    crushrole,
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &payload); err != nil {
		c.Log.Error(err)
	}
	return
}

func (c *Cluster) ZonesList() (payload []string) {
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd crush rule ls",
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &payload); err != nil {
		c.Log.Error(err)
	}
	return
}

// TODO
func (c *Cluster) GetPoolStat(pool string) (used Size, provision Size) {
	ioctx, err := c.Conn.OpenIOContext(pool)
	if err != nil {
		c.Log.Error(err)
	}
	defer ioctx.Destroy()
	stat, err := ioctx.GetPoolStats()
	if err != nil {
		c.Log.Error(err)
	}
	used.InBytes = stat.Num_bytes
	provision.InBytes = stat.Num_bytes * stat.Num_object_copies
	used.Convert()
	provision.Convert()
	return
}

func (c *Cluster) ZonesDump() map[string]interface{} {
	var payload map[string]interface{}
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd df",
			VAR:    "tree",
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &payload); err != nil {
		c.Log.Error(err)
	}
	return payload
}

func (c *Cluster) CrushRoleDump() map[string]interface{} {
	var payload map[string]interface{}
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd crush dump",
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &payload); err != nil {
		c.Log.Error(err)
	}
	return payload
}

func (c *Cluster) GetVersion() (int, error) {
	var payload map[string]string
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "version",
			FORMAT: "json",
		})
	if err != nil {
		return 0, err
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		return 0, err
	}

	if err = json.Unmarshal(bcontent, &payload); err != nil {
		return 0, err
	}
	data := payload["version"]
	version, err := strconv.Atoi(strings.Split(strings.Split(data, " ")[2], ".")[0])
	if err != nil {
		return 0, err
	}
	return version, nil
}

// stat, err := c.Conn.GetClusterStats()
// if err != nil {
// 	log.Fatal(err)
// }
// c.Log.Info("AVAI ", core.PrettyByteSize(int(stat.Kb_avail)*1024))
// c.Log.Info("USED ", core.PrettyByteSize(int(stat.Kb_used)*1024))

// for _, pool := range c.Pools {
// 	payload, err := c.GetZoneByPoolName(pool)
// 	if err != nil {
// 		c.Log.Errorf("Some error: %s \n", err)
// 	}
// 	c.Log.Debugln(payload)
// }
