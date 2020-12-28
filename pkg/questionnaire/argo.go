package questionnaire

import (
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
)

func AskAboutPass(installOptions *install.CmdOptions) error {
	installOptions.Argo.Password = askAboutPass()
	return nil
}

func askAboutPass() string {
	var firstPassword string
	_ = prompt.InputPassword(&firstPassword, "Please specify root password for ArgoCD")
	return firstPassword
}
