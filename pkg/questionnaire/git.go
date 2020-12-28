package questionnaire

import (
	"fmt"
	"github.com/codefresh-io/argocd-listener/installer/pkg/logger"
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/go-sdk/pkg/codefresh"
)

func AskAboutGitRepo(installOptions *install.CmdOptions) error {
	if installOptions.Git.Integration == "" || installOptions.Git.Auth.Pass == "" {
		return nil
	}
	_ = prompt.InputWithDefault(&installOptions.Git.RepoUrl, "Please specify url to your manifest repository to add to ArgoCD", "https://github.com/argoproj/argocd-example-apps")
	return nil
}

func AskAboutGitContext(installOptions *install.CmdOptions, contexts *[]codefresh.ContextPayload) error {
	if len(*contexts) < 1 {
		return nil
	}

	var passwords = make(map[string]string)
	var types = make(map[string]string)
	var list []string
	for _, v := range *contexts {
		types[v.Metadata.Name] = v.Spec.Data.Auth.Type
		passwords[v.Metadata.Name] = v.Spec.Data.Auth.Password
		list = append(list, v.Metadata.Name)
	}

	if len(list) == 1 {
		installOptions.Git.Integration = list[0]
	} else {
		_, installOptions.Git.Integration = prompt.Select(list, "Select Git context")
	}

	logger.Info(fmt.Sprintf("Use \"%s\" git integration for integrate with manifest repo", installOptions.Git.Integration))

	installOptions.Git.Auth.Type = types[installOptions.Git.Integration]
	installOptions.Git.Auth.Pass = passwords[installOptions.Git.Integration]

	return nil
}
