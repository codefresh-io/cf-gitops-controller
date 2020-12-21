package questionnaire

import (
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/go-sdk/pkg/codefresh"
)

func AskAboutClusters(installOptions *install.CmdOptions, clusters []*codefresh.ClusterMinified) error {
	if len(clusters) < 1 {
		return nil
	}

	return nil
}
