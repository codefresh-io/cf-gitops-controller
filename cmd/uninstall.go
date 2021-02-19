package cmd

import (
	"errors"
	"fmt"
	cfEventSender "github.com/codefresh-io/argocd-listener/installer/pkg/cf_event_sender"
	"github.com/codefresh-io/argocd-listener/installer/pkg/kube"
	"github.com/codefresh-io/argocd-listener/installer/pkg/logger"
	agentUninstallPkg "github.com/codefresh-io/argocd-listener/installer/pkg/uninstall"
	agentUninstaller "github.com/codefresh-io/argocd-listener/installer/pkg/uninstall/handler"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/questionnaire"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/user"
	"path"
)

var uninstallCmdOptions = install.CmdOptions{}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall gitops codefresh",
	Long:  `Uninstall gitops codefresh`,
	RunE: func(cmd *cobra.Command, args []string) error {

		_ = questionnaire.AskAboutKubeContext(&uninstallCmdOptions)
		kubeOptions := uninstallCmdOptions.Kube
		kubeClient, err := kube.New(&kube.Options{
			ContextName:      kubeOptions.Context,
			Namespace:        kubeOptions.Namespace,
			PathToKubeConfig: kubeOptions.ConfigPath,
		})

		if err != nil {
			return failUninstall(fmt.Sprintf("Can't create kube client: \"%s\"", err.Error()))
		}

		_ = questionnaire.AskAboutNamespace(&uninstallCmdOptions, kubeClient)
		_ = kubeClient.CreateNamespace(uninstallCmdOptions.Kube.Namespace)

		_ = questionnaire.AskAboutManifest(&uninstallCmdOptions)
		err = kubeClient.DeleteObjects(uninstallCmdOptions.Kube.ManifestPath)
		if err != nil {
			return failUninstall(fmt.Sprintf("Can't delete kube objects: \"%s\"", err.Error()))
		}

		uninstallHandler := agentUninstaller.New(initAgentUninstallOptions(&uninstallCmdOptions))
		err = uninstallHandler.Run()
		if err != nil {
			return failUninstall(fmt.Sprintf("Can't uninstall argocd agent: \"%s\"", err.Error()))
		}
		successMsg := fmt.Sprintf("Codefresh gitops controller uninstallation finished successfully")
		logger.Success(successMsg)
		eventSender := cfEventSender.New(cfEventSender.EVENT_CONTROLLER_UNINSTALL)
		eventSender.Success(successMsg)
		return nil
	},
}

func initAgentUninstallOptions(uninstallCmdOptions *install.CmdOptions) agentUninstallPkg.CmdOptions {
	var agentUninstallOptions agentUninstallPkg.CmdOptions
	agentUninstallOptions.Kube.Namespace = uninstallCmdOptions.Kube.Namespace
	agentUninstallOptions.Kube.Context = uninstallCmdOptions.Kube.Context
	agentUninstallOptions.Kube.InCluster = uninstallCmdOptions.Kube.InCluster
	agentUninstallOptions.Kube.ConfigPath = uninstallCmdOptions.Kube.ConfigPath
	return agentUninstallOptions
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	flags := uninstallCmd.Flags()

	flags.StringVar(&uninstallCmdOptions.Kube.Namespace, "kube-namespace", viper.GetString("kube-namespace"), "Namespace in Kubernetes cluster")
	flags.StringVar(&uninstallCmdOptions.Kube.ManifestPath, "install-manifest", "", "Url of argocd install manifest")

	flags.BoolVar(&uninstallCmdOptions.Kube.InCluster, "in-cluster", false, "Set flag if argocd is been installed from inside a cluster")

	var kubeConfigPath string
	currentUser, _ := user.Current()
	if currentUser != nil {
		kubeConfigPath = os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = path.Join(currentUser.HomeDir, ".kube", "config")
		}
	}

	flags.StringVar(&uninstallCmdOptions.Kube.Context, "kube-context-name", viper.GetString("kube-context"), "Name of the kubernetes context on which Argo agent should be installed (default is current-context) [$KUBE_CONTEXT]")
	flags.StringVar(&uninstallCmdOptions.Kube.ConfigPath, "kubeconfig", kubeConfigPath, "Path to kubeconfig file (default is $HOME/.kube/config)")
}

func failUninstall(msg string) error {
	eventSender := cfEventSender.New(cfEventSender.EVENT_CONTROLLER_UNINSTALL)
	eventSender.Fail(msg)
	return errors.New(msg)
}
