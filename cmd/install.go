package cmd

import (
	"errors"
	"fmt"
	argo "github.com/codefresh-io/argocd-sdk/pkg/api"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/kube"
	"github.com/codefresh-io/cf-gitops-controller/pkg/logger"
	"github.com/codefresh-io/cf-gitops-controller/pkg/questionnaire"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/user"
	"path"
)

var DEFAULT_USER_NAME = "admin"
var FAILED = "FAILED"
var installCmdOptions = install.CmdOptions{}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gitops codefresh",
	Long:  `Install gitops codefresh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Success("This installer will guide you through the Codefresh Gitops controller installation")

		_ = questionnaire.AskAboutKubeContext(&installCmdOptions)
		kubeOptions := installCmdOptions.Kube
		kubeClient, err := kube.New(&kube.Options{
			ContextName:      kubeOptions.Context,
			Namespace:        kubeOptions.Namespace,
			PathToKubeConfig: kubeOptions.ConfigPath,
		})
		if err != nil {
			return err
		}

		_ = questionnaire.AskAboutNamespace(&installCmdOptions, kubeClient)
		namespaceIsExist, err := kubeClient.NamespaceExists()
		if namespaceIsExist != true {
			logger.Info(fmt.Sprintf("Creating namespace \"%s\"...", installCmdOptions.Kube.Namespace))
			err = kubeClient.CreateNamespace()
			if err != nil {
				return failInstallation(fmt.Sprintf("Can't create namespace %s: \"%s\"", installCmdOptions.Kube.Namespace, err.Error()))
			}
		}

		_ = questionnaire.AskAboutManifest(&installCmdOptions)
		logger.Info(fmt.Sprint("Creating argocd resources..."))
		err = kubeClient.CreateDeployments(installCmdOptions.Kube.ManifestPath)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't create argocd resources: \"%s\"", err.Error()))
		}

		logger.Info(fmt.Sprint("Changing service type to \"LoadBalancer\"..."))

		argocdServer, err := kubeClient.GetDeployments("app.kubernetes.io/name=argocd-server")
		if err == nil {
			err = kubeClient.UpdateDeployments(argocdServer)
		}
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't change service type to LoadBalancer: \"%s\"", err.Error()))
		}

		logger.Info(fmt.Sprint("Getting argocd ip address..."))
		argoHost, err := kubeClient.GetArgoServerHost()
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't change service type to LoadBalancer: \"%s\"", err.Error()))
		}

		logger.Info(fmt.Sprint("Getting autogenerated password..."))
		autogenerated, err := kubeClient.GetAutogeneratedPassword()
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get autogenerated password: \"%s\"", err.Error()))
		}

		logger.Info(fmt.Sprint("Getting argocd token..."))
		token, err := argo.GetToken(DEFAULT_USER_NAME, autogenerated, argoHost)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argo token: \"%s\"", err.Error()))
		}

		argoClientOptions := argo.ClientOptions{Auth: argo.AuthOptions{Token: token}, Host: argoHost}
		argoApi := argo.New(&argoClientOptions)

		_ = questionnaire.AskAboutPass(&installCmdOptions)
		logger.Info(fmt.Sprint("Updating admin password..."))
		err = argoApi.Auth().UpdatePassword(argo.UpdatePasswordOpt{
			CurrentPassword: autogenerated,
			UserName:        DEFAULT_USER_NAME,
			NewPassword:     installCmdOptions.Argo.Password,
		})
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't update user pass: \"%s\"", err.Error()))
		}

		logger.Success(fmt.Sprint("Successfully installed codefresh gitops controller"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	flags := installCmd.Flags()

	flags.StringVar(&installCmdOptions.Argo.Password, "set-argo-password", "", "Set password for admin user of new argocd installation")
	flags.StringVar(&installCmdOptions.Kube.Namespace, "kube-namespace", "", "Namespace in Kubernetes cluster")
	flags.StringVar(&installCmdOptions.Kube.ManifestPath, "install-manifest", "", "Url of argocd install manifest")

	var kubeConfigPath string
	currentUser, _ := user.Current()
	if currentUser != nil {
		kubeConfigPath = os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = path.Join(currentUser.HomeDir, ".kube", "config")
		}
	}

	flags.StringVar(&installCmdOptions.Kube.ConfigPath, "kube-config-path", kubeConfigPath, "Path to kubeconfig file (default is $HOME/.kube/config)")
	flags.StringVar(&installCmdOptions.Kube.Context, "kube-context-name", viper.GetString("kube-context"), "Name of the kubernetes context on which Argo agent should be installed (default is current-context) [$KUBE_CONTEXT]")

}

func sendControllerInstalledEvent(status string, msg string) {

}

func failInstallation(msg string) error {
	sendControllerInstalledEvent(FAILED, msg)
	return errors.New(msg)
}
