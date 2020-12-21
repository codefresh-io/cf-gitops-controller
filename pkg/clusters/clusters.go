package clusters

import (
	"encoding/base64"
	"fmt"
	argo "github.com/codefresh-io/argocd-sdk/pkg/api"
	"github.com/codefresh-io/cf-gitops-controller/pkg/logger"
	"github.com/codefresh-io/go-sdk/pkg/codefresh"
)

func FilterClusters(clusters []*codefresh.ClusterMinified) []*codefresh.ClusterMinified {
	filteredClusters := []*codefresh.ClusterMinified{}
	for _, cluster := range clusters {
		if cluster.Provider == "local" {
			filteredClusters = append(filteredClusters, cluster)
		}
	}
	return filteredClusters
}

func ImportFromCodefresh(clusters []string, cfClustersApi codefresh.IClusterAPI, argoClustersApi argo.ClusterApi) error {
	for _, clusterSelector := range clusters {
		cluster, err := cfClustersApi.GetClusterCredentialsByAccountId(clusterSelector)
		if err != nil {
			return err
		}

		bearer, err := base64.StdEncoding.DecodeString(cluster.Auth.Bearer)
		if err != nil {
			return err
		}

		_, err = argoClustersApi.CreateCluster(argo.ClusterOpt{
			Name:   clusterSelector,
			Server: cluster.Url,
			Config: argo.ClusterConfig{
				BearerToken: string(bearer),
				TlsClientConfig: argo.TlsClientConfig{
					CaData:   cluster.Ca,
					Insecure: false,
				},
			},
		})
		if err != nil {
			return err
		}
		logger.Success(fmt.Sprintf("Successfull created cluster \"%s\"", clusterSelector))
	}

	return nil
}
