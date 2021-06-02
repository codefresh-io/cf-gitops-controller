package questionnaire

import (
	"github.com/codefresh-io/argocd-listener/installer/pkg/kube"
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
)

func AskAboutNamespace(installOptions *install.CmdOptions, kubeClient kube.Kube) error {
	if installOptions.Kube.Namespace == "" {
		namespaces, err := kubeClient.GetNamespaces()
		if err != nil {
			err = prompt.NewPrompt().InputWithDefault(&installOptions.Kube.Namespace, "Kubernetes namespace to install", "default")
			if err != nil {
				return err
			}
		} else {
			err, selectedNamespace := prompt.NewPrompt().Select(namespaces, "Select the namespace")
			if err != nil {
				return err
			}
			installOptions.Kube.Namespace = selectedNamespace
		}
	}
	return nil
}
