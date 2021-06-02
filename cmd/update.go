package cmd

import (
	"fmt"
	"github.com/codefresh-io/argocd-listener/installer/pkg/logger"
	agentUpdatePkg "github.com/codefresh-io/argocd-listener/installer/pkg/update"
	agentUpdater "github.com/codefresh-io/argocd-listener/installer/pkg/update/handler"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/kube"
	"github.com/codefresh-io/cf-gitops-controller/pkg/questionnaire"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/user"
	"path"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update gitops codefresh",
	Long:  `Update gitops codefresh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = questionnaire.AskAboutCodefreshCredentials(&installCmdOptions)
		// kube context
		_ = questionnaire.AskAboutKubeContext(&installCmdOptions)
		kubeOptions := installCmdOptions.Kube
		kubeClient, err := kube.New(&kube.Options{
			ContextName:      kubeOptions.Context,
			Namespace:        kubeOptions.Namespace,
			PathToKubeConfig: kubeOptions.ConfigPath,
		})
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't create kube client: \"%s\"", err.Error()))
		}

		// namespace
		_ = questionnaire.AskAboutNamespace(&installCmdOptions, kubeClient)
		err = kubeClient.CreateNamespace(installCmdOptions.Kube.Namespace)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't create namespace %s: \"%s\"", installCmdOptions.Kube.Namespace, err.Error()))
		}

		updateHandler := agentUpdater.New(initAgentUpdateOptions(&installCmdOptions), agentVersion)
		err = updateHandler.Run()
		if err != nil {
			return failUninstall(fmt.Sprintf("Can't update argocd agent: \"%s\"", err.Error()))
		}

		logger.Success(fmt.Sprint("Successfully updated codefresh gitops controller"))
		return nil
	},
}

func initAgentUpdateOptions(installCmdOptions *install.CmdOptions) agentUpdatePkg.CmdOptions {
	var agentUpdateOptions agentUpdatePkg.CmdOptions

	agentUpdateOptions.Kube.Namespace = installCmdOptions.Kube.Namespace
	agentUpdateOptions.Kube.Context = installCmdOptions.Kube.Context
	agentUpdateOptions.Kube.ConfigPath = installCmdOptions.Kube.ConfigPath

	return agentUpdateOptions
}

func init() {
	rootCmd.AddCommand(updateCmd)
	flags := updateCmd.Flags()

	var kubeConfigPath string
	currentUser, _ := user.Current()
	if currentUser != nil {
		kubeConfigPath = os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = path.Join(currentUser.HomeDir, ".kube", "config")
		}
	}
	flags.StringVar(&installCmdOptions.Kube.Namespace, "kube-namespace", "argocd", "Namespace in Kubernetes cluster")
	flags.StringVar(&installCmdOptions.Kube.ConfigPath, "kubeconfig", kubeConfigPath, "Path to kubeconfig file (default is $HOME/.kube/config)")
	flags.StringVar(&installCmdOptions.Kube.Context, "kube-context-name", viper.GetString("kube-context"), "Name of the kubernetes context on which Argo agent should be installed (default is current-context) [$KUBE_CONTEXT]")
}
