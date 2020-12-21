package helper

import "github.com/codefresh-io/go-sdk/pkg/codefresh"

//type Helper interface {
//	FilterClusters([]*codefresh.ClusterMinified) []*codefresh.ClusterMinified
//}

func FilterClusters(clusters []*codefresh.ClusterMinified) []*codefresh.ClusterMinified {
	filteredClusters := []*codefresh.ClusterMinified{}
	for _, cluster := range clusters {
		if cluster.Provider == "local" {
			filteredClusters = append(filteredClusters, cluster)
		}
	}
	return filteredClusters
}
