package adapters

import (
	"fmt"
	"strconv"
	"text/tabwriter"
	"github.com/getcarina/libcarina"
	"github.com/pkg/errors"
	"time"
)

type MakeSwarm struct {
	Credentials UserCredentials
	Output *tabwriter.Writer
}

const httpTimeout = time.Second * 15

func (carina *MakeSwarm) LoadCredentials(credentials UserCredentials) error {
	carina.Credentials = credentials
	return nil
}

func (carina *MakeSwarm) authenticate() (*libcarina.ClusterClient, error) {
	fmt.Println("[DEBUG][make-swarm] Authenticating...")
	carinaClient, err := libcarina.NewClusterClient(carina.Credentials.Endpoint, carina.Credentials.UserName, carina.Credentials.Secret)
	if err == nil {
		carinaClient.Client.Timeout = httpTimeout
	}
	return carinaClient, err
}

func (carina *MakeSwarm) ListClusters() error {
	carinaClient, err := carina.authenticate()
	if err != nil {
		return errors.Wrap(err, "[make-swarm] Authentication failed")
	}

	fmt.Println("[DEBUG][make-swarm] Listing clusters...")
	clusterList, err := carinaClient.List()
	if err != nil {
		return err
	}

	err = carina.writeClusterHeader()
	if err != nil {
		return err
	}

	for _, cluster := range clusterList {
		err = carina.writeCluster(&cluster)
		if err != nil {
			return err
		}
	}

	return err
}

func (carina *MakeSwarm) writeCluster(cluster *libcarina.Cluster) error {
	fields := []string{
		cluster.ClusterName,
		cluster.Flavor,
		strconv.FormatInt(cluster.Nodes.Int64(), 10),
		strconv.FormatBool(cluster.AutoScale),
		cluster.Status,
	}
	return writeRow(carina.Output, fields)
}

func (carina *MakeSwarm) writeClusterHeader() error {
	headerFields := []string{
		"ClusterName",
		"Flavor",
		"Nodes",
		"AutoScale",
		"Status",
	}
	return writeRow(carina.Output, headerFields)
}