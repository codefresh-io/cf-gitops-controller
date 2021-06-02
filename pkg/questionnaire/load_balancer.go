package questionnaire

import (
	"errors"
	"fmt"
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/kube"
)

func initLoadBalancer(kubeClient kube.Kube) error {
	// load balancer
	argocdServer, err := kubeClient.GetService("app.kubernetes.io/name=argocd-server")
	if err != nil {
		return errors.New(fmt.Sprintf("Can't get argocd server: \"%s\"", err.Error()))
	}
	argocdServer.Spec.Type = "LoadBalancer"
	err = kubeClient.UpdateService(argocdServer)
	if err != nil {
		return errors.New(fmt.Sprintf("Can't change service type to LoadBalancer: \"%s\"", err.Error()))
	}
	return nil
}

func AskAboutLoadBalancer(installOptions *install.CmdOptions, kubeClient kube.Kube) error {

	if installOptions.Controller.LoadBalancer {
		return initLoadBalancer(kubeClient)
	}

	_, loadBalancer := prompt.NewPrompt().Confirm("Would you like to expose ArgoCD with LoadBalancer? ( This is required when using Codefresh steps )")
	if loadBalancer {
		return initLoadBalancer(kubeClient)
	}

	return nil
}
