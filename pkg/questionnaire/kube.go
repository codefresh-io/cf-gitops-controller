package questionnaire

import (
	"github.com/codefresh-io/argocd-listener/installer/pkg/kube"
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
)

func AskAboutKubeContext(installOptions *install.CmdOptions) error {
	kubeOptions := installOptions.Kube
	kubeConfigPath := installOptions.Kube.ConfigPath
	if kubeOptions.Context == "" {
		contexts, err := kube.GetAllContexts(kubeConfigPath)
		if err != nil {
			return err
		}

		if len(contexts) == 1 {
			kubeOptions.Context = contexts[0]
		} else {
			_, selectedContext := prompt.NewPrompt().Select(contexts, "Select Kubernetes context")
			kubeOptions.Context = selectedContext
		}

	}
	installOptions.Kube.Context = kubeOptions.Context
	return nil
}

func AskAboutManifest(installOptions *install.CmdOptions) error {
	// dont need ask for now, customer can pass it use params
	installOptions.Kube.ManifestPath = "https://raw.githubusercontent.com/codefresh-io/argo-cd/stable/manifests/install.yaml"
	return nil
	//return prompt.InputWithDefault(&installOptions.Kube.ManifestPath, "Install manifest path/url", "https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml")
}
