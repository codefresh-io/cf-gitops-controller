package questionnaire

import (
	"github.com/codefresh-io/cf-gitops-controller/pkg/cliconfig"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
)

func AskAboutCodefreshCredentials(installOptions *install.CmdOptions) error {
	if installOptions.Codefresh.Auth.Token == "" || installOptions.Codefresh.Host == "" {
		config, err := cliconfig.GetCurrentConfig()
		if err != nil {
			return err
		}
		installOptions.Codefresh.Auth.Token = config.Token
		installOptions.Codefresh.Host = config.Url
	}
	return nil
}
