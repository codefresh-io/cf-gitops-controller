package questionnaire

import (
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/prompt"
)

func AskAboutPass(installOptions *install.CmdOptions) error {
	installOptions.Argo.Password = askAboutPass()
	return nil
}

func askAboutPass() string {
	var firstPassword string
	_ = prompt.InputPassword(&firstPassword, "New argocd password")
	return firstPassword
}
