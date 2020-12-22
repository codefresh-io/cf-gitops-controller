package cmd

import (
	"errors"
	"fmt"
	argo "github.com/codefresh-io/argocd-sdk/pkg/api"
	"github.com/codefresh-io/cf-gitops-controller/pkg/clusters"
	"github.com/codefresh-io/cf-gitops-controller/pkg/git"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/kube"
	"github.com/codefresh-io/cf-gitops-controller/pkg/logger"
	"github.com/codefresh-io/cf-gitops-controller/pkg/questionnaire"
	"github.com/codefresh-io/go-sdk/pkg/codefresh"
	"github.com/janeczku/go-spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/user"
	"path"
	"time"
)

var DEFAULT_USER_NAME = "admin"
var FAILED = "FAILED"
var installCmdOptions = install.CmdOptions{}

func retrieveArgoHost(kubeClient kube.Kube) (string, error) {
	var argoHost string
	var err error
	start := time.Now()
	s := spinner.StartNew("Getting argocd ip address...")
	for {
		argoHost, err = kubeClient.GetArgoServerHost()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
		if time.Now().Sub(start).Minutes() > 2 {
			return "", errors.New("Failed to retrieve argocd host")
		}
	}
	s.Stop()
	return argoHost, err
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gitops codefresh",
	Long:  `Install gitops codefresh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Success("This installer will guide you through the Codefresh Gitops controller installation")

		_ = questionnaire.AskAboutCodefreshCredentials(&installCmdOptions)

		codefreshApi := codefresh.New(&codefresh.ClientOptions{
			Host: installCmdOptions.Codefresh.Host,
			Auth: codefresh.AuthOptions{
				Token: installCmdOptions.Codefresh.Auth.Token,
			},
		})

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

		// manifest
		_ = questionnaire.AskAboutManifest(&installCmdOptions)
		logger.Info(fmt.Sprint("Creating argocd resources..."))
		err = kubeClient.CreateObjects(installCmdOptions.Kube.ManifestPath)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't create argocd resources: \"%s\"", err.Error()))
		}

		// load balancer
		argocdServer, err := kubeClient.GetService("app.kubernetes.io/name=argocd-server")
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argocd server: \"%s\"", err.Error()))
		}
		argocdServer.Spec.Type = "LoadBalancer"
		err = kubeClient.UpdateService(argocdServer)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't change service type to LoadBalancer: \"%s\"", err.Error()))
		}

		//argo ghost
		argoHost, err := retrieveArgoHost(kubeClient)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't retrieve argo host: \"%s\"", err.Error()))
		}

		// default pass
		logger.Info(fmt.Sprint("Getting autogenerated password..."))
		autogenerated, err := kubeClient.GetAutogeneratedPassword()
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get autogenerated password: \"%s\"", err.Error()))
		}

		// getting token
		logger.Info(fmt.Sprint("\nGetting argocd token..."))
		token, err := argo.GetToken(DEFAULT_USER_NAME, autogenerated, argoHost)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argo token: \"%s\"", err.Error()))
		}
		argoClientOptions := argo.ClientOptions{Auth: argo.AuthOptions{Token: token}, Host: argoHost}
		argoApi := argo.New(&argoClientOptions)

		// changing pass
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

		// update argo client @todo - only if user add clusters or repo
		logger.Info(fmt.Sprint("Updating argo client..."))
		token, err = argo.GetToken(DEFAULT_USER_NAME, installCmdOptions.Argo.Password, argoHost)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argo token: \"%s\"", err.Error()))
		}
		argoClientOptions = argo.ClientOptions{Auth: argo.AuthOptions{Token: token}, Host: argoHost}
		argoApi = argo.New(&argoClientOptions)

		//clusters
		logger.Info(fmt.Sprint("Getting argocd clusters..."))
		clustersList, err := clusters.GetAvailableClusters(codefreshApi.Clusters())
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argocd clusters: \"%s\"", err.Error()))
		}
		_ = questionnaire.AskAboutClusters(&installCmdOptions, clustersList)
		err = clusters.ImportFromCodefresh(installCmdOptions.Codefresh.Clusters, codefreshApi.Clusters(), argoApi.Clusters())
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't import clusters: \"%s\"", err.Error()))
		}

		// git repo
		contexts, err := git.GetAvailableContexts(codefreshApi.Contexts())
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get git contexts: \"%s\"", err.Error()))
		}
		_ = questionnaire.AskAboutGitContext(&installCmdOptions, contexts)
		_ = questionnaire.AskAboutGitRepo(&installCmdOptions)
		if installCmdOptions.Git.RepoUrl != "" {
			logger.Info(fmt.Sprint("Creating repositories..."))
			err = argoApi.Repository().CreateRepository(argo.CreateRepositoryOpt{
				Repo:     installCmdOptions.Git.RepoUrl,
				Username: installCmdOptions.Git.Auth.Pass,
				Password: installCmdOptions.Git.Auth.Pass,
			})
			if err != nil {
				// @todo - retry url passing
				return failInstallation(fmt.Sprintf("Can't manage access to git repo: \"%s\"", err.Error()))
			}
		}

		logger.Success(fmt.Sprintf("Successfully installed codefresh gitops controller, host: %s", argoHost))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	flags := installCmd.Flags()

	flags.StringVar(&installCmdOptions.Codefresh.Host, "codefresh-host", "", "Codefresh host")
	flags.StringVar(&installCmdOptions.Codefresh.Auth.Token, "codefresh-token", "", "Codefresh api token")
	flags.StringArrayVar(&installCmdOptions.Codefresh.Clusters, "codefresh-clusters", make([]string, 0), "")

	flags.StringVar(&installCmdOptions.Argo.Password, "argo-password", "", "Set password for admin user of new argocd installation")

	flags.StringVar(&installCmdOptions.Kube.Namespace, "kube-namespace", "argocd", "Namespace in Kubernetes cluster")
	flags.StringVar(&installCmdOptions.Kube.ManifestPath, "install-manifest", "", "Url of argocd install manifest")

	var kubeConfigPath string
	currentUser, _ := user.Current()
	if currentUser != nil {
		kubeConfigPath = os.Getenv("KUBECONFIG")
		if kubeConfigPath == "" {
			kubeConfigPath = path.Join(currentUser.HomeDir, ".kube", "config")
		}
	}

	flags.StringVar(&installCmdOptions.Kube.ConfigPath, "kubeconfig", kubeConfigPath, "Path to kubeconfig file (default is $HOME/.kube/config)")
	flags.StringVar(&installCmdOptions.Kube.Context, "kube-context-name", viper.GetString("kube-context"), "Name of the kubernetes context on which Argo agent should be installed (default is current-context) [$KUBE_CONTEXT]")

	flags.StringVar(&installCmdOptions.Git.Integration, "git-integration", "", "Name of git integration in Codefresh")
	flags.StringVar(&installCmdOptions.Git.RepoUrl, "git-repo-url", "", "Url to git manifest repo")

}

func sendControllerInstalledEvent(status string, msg string) {

}

func failInstallation(msg string) error {
	sendControllerInstalledEvent(FAILED, msg)
	return errors.New(msg)
}
