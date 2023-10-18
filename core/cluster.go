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
	METHOD           string = "output_method"
	CRUSHROLE        string = "crush_rule"
	CRHUSHROLELEGASY string = "crush_ruleset"
)

type Pool struct {
	Name     string
	ZoneId   int
	ZoneName string
	Stat     Stat
}

// type Zone struct {
// 	Name string
// 	Stat Stat
// }

type Cluster struct {
	ID           int
	Name         string
	Conn         *rados.Conn
	IOctx        *rados.IOContext
	Pools        []Pool
	Zones        []Zone
	RootZones    []Zone
	Config       ClusterConf
	cephConfPath string
	Log          *logrus.Logger
	Version      int
}

type NodesStat struct {
	Nodes []OsdStat `json:"nodes"`
	// Stat  Summary   `json:"summary"`
}

type OsdStat struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	TypeID  int    `json:"type_id"`
	TotalKB uint64 `json:"kb"`
	UsedKB  uint64 `json:"kb_used"`
	AvailKB uint64 `json:"kb_avail"`
	Stat    Stat
	// Utilization float64 `json:"average_utilization"`
}

type Zone struct {
	ID         int    `json:"rule_id"`
	Name       string `json:"rule_name"`
	PublicName string
	Type       int `json:"type"`
	IsRoot     bool
	Steps      []struct {
		ItemName string `json:"item_name"`
	} `json:"steps"`
	Stat Stat
}

type Stat struct {
	Total Size `json:"total"`
	Used  Size `json:"used"`
	Avail Size `json:"avail"`
	Provi Size `json:"provision"`
	Free  Size `json:"free"`
}

func NewCluster(clusterName, credsPath string, conf ClusterConf) (*Cluster, error) {
	c := &Cluster{
		Name:         clusterName,
		Config:       conf,
		cephConfPath: path.Join(credsPath, conf.Conf),
		Log:          NewLogger(clusterName),
	}

	conn, err := rados.NewConnWithUser(c.Config.User)
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
			Name:     pool,
			ZoneId:   int(zoneId),
			ZoneName: zoneName,
			Stat: Stat{
				Used:  used,
				Provi: provision,
			},
		})

	}
	// c.Zones = zones
	// c.Close()
	return c, nil
}

func (c *Cluster) Open() error {
	if err := c.Conn.Connect(); err != nil {
		return err
	}
	c.Log.Debug("Connect")
	return nil
}

func (c *Cluster) Close() {
	c.Conn.Shutdown()
	c.Log.Debug("Disconnect")
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

func (c *Cluster) GetZonesStat() {
	for _, node := range c.NodesDump().Nodes {
		if node.Type == "root" {
			for _, zone := range c.Zones {
				// if node.ID == zone.ID {
				zone.IsRoot = true
				zone.Stat.Total.Set(Percent(75, int(node.TotalKB*1024/3)))
				zone.Stat.Avail.Set(float64(node.AvailKB * 1024))
				zone.Stat.Used.Set(float64(node.UsedKB*1024) / 3)

				zone.PublicName = node.Name
				// }
				c.RootZones = append(c.RootZones, zone)
			}

		}
	}
}

func (c *Cluster) Calculate() {
	var tmpZones []Zone
	for _, zone := range c.RootZones {
		for _, pool := range c.Pools {
			if pool.ZoneName == zone.Name {
				zone.Stat.Provi.PlusBytes(pool.Stat.Provi.InBytes)
			}
		}
		zone.Stat.Provi.Convert()
		zone.Stat.Free.Set(zone.Stat.Total.InBytes - zone.Stat.Provi.InBytes)

		tmpZones = append(tmpZones, zone)
	}
	c.RootZones = tmpZones
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
	used.InBytes = float64(stat.Num_bytes)
	provision.InBytes = float64(stat.Num_bytes * stat.Num_object_copies)
	used.Convert()
	provision.Convert()
	return
}

func (c *Cluster) NodesDump() (nodesStat NodesStat) {
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd df",
			METHOD: "tree",
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &nodesStat); err != nil {
		c.Log.Error(err)
	}
	return nodesStat
}

func (c *Cluster) ZonesList() (rules []Zone) {
	b, err := json.Marshal(
		map[string]interface{}{
			PREFIX: "osd crush rule dump",
			// VAR:    "dump",
			FORMAT: "json",
		})
	if err != nil {
		c.Log.Error(err)
	}
	bcontent, _, err := c.Conn.MonCommand(b)
	if err != nil {
		c.Log.Error(err)
	}

	if err = json.Unmarshal(bcontent, &rules); err != nil {
		c.Log.Error(err)
	}
	return rules
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
