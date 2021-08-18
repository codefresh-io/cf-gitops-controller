package cmd

import (
	"errors"
	"fmt"
	"github.com/codefresh-io/argocd-listener/agent/pkg/infra/store"
	cfEventSender "github.com/codefresh-io/argocd-listener/installer/pkg/cfeventsender"
	agentInstaller "github.com/codefresh-io/argocd-listener/installer/pkg/install"
	agentInstallPkg "github.com/codefresh-io/argocd-listener/installer/pkg/install/entity"
	"github.com/codefresh-io/argocd-listener/installer/pkg/logger"
	"github.com/codefresh-io/argocd-listener/installer/pkg/prompt"
	argoSdk "github.com/codefresh-io/argocd-sdk/pkg/api"
	argo "github.com/codefresh-io/cf-gitops-controller/pkg/argo"
	"github.com/codefresh-io/cf-gitops-controller/pkg/clusters"
	"github.com/codefresh-io/cf-gitops-controller/pkg/git"
	"github.com/codefresh-io/cf-gitops-controller/pkg/install"
	"github.com/codefresh-io/cf-gitops-controller/pkg/kube"
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

var agentVersion = ""
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

		store.SetCodefresh(installCmdOptions.Codefresh.Host, installCmdOptions.Codefresh.Auth.Token, "")

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

		_ = questionnaire.AskAboutLoadBalancer(&installCmdOptions, kubeClient)

		//argo ghost
		argoHost, err := retrieveArgoHost(kubeClient)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't retrieve argo host: \"%s\"", err.Error()))
		}
		installCmdOptions.Argo.Host = argoHost

		// default pass
		logger.Info(fmt.Sprint("Getting autogenerated password..."))
		autogenerated, err := kubeClient.GetAutogeneratedPassword()
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get autogenerated password: \"%s\"", err.Error()))
		}

		// getting token
		logger.Info(fmt.Sprint("\nGetting argocd token..."))

		token, err := questionnaire.NewArgocdTokenQuestion(installCmdOptions.Argo.Username, autogenerated, argoHost).Ask()

		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argo token: \"%s\"", err.Error()))
		}

		argoClientOptions := argoSdk.ClientOptions{Auth: argoSdk.AuthOptions{Token: token}, Host: argoHost}
		argoApi := argoSdk.New(&argoClientOptions)

		// changing pass
		_ = questionnaire.AskAboutPass(&installCmdOptions)
		logger.Info(fmt.Sprint("\nUpdating admin password..."))
		err = argoApi.Auth().UpdatePassword(argoSdk.UpdatePasswordOpt{
			CurrentPassword: autogenerated,
			UserName:        installCmdOptions.Argo.Username,
			NewPassword:     installCmdOptions.Argo.Password,
		})
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't update user pass: \"%s\"", err.Error()))
		}

		// update argo client @todo - only if user add clusters or repo
		logger.Info(fmt.Sprint("Updating argo client..."))
		token, err = argoSdk.GetToken(installCmdOptions.Argo.Username, installCmdOptions.Argo.Password, argoHost)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't get argo token: \"%s\"", err.Error()))
		}
		installCmdOptions.Argo.Token = token
		argoClientOptions = argoSdk.ClientOptions{Auth: argoSdk.AuthOptions{Token: token}, Host: argoHost}
		argoApi = argoSdk.New(&argoClientOptions)

		_, addClusters := prompt.NewPrompt().Confirm("Would you like to integrate clusters from your account to ArgoCD?")

		if addClusters {
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
		}

		_, addManifestRepo := prompt.NewPrompt().Confirm("Would you like to integrate git context for manifest repo from your account to ArgoCD?")
		if addManifestRepo {
			// git repo
			contexts, err := git.GetAvailableContexts(codefreshApi.Contexts())
			if err != nil {
				return failInstallation(fmt.Sprintf("Can't get git contexts: \"%s\"", err.Error()))
			}
			_ = questionnaire.AskAboutGitContext(&installCmdOptions, contexts)
			_ = questionnaire.AskAboutGitRepo(&installCmdOptions)
			if installCmdOptions.Git.RepoUrl != "" {
				logger.Info(fmt.Sprint("Creating repositories..."))
				err = argoApi.Repository().CreateRepository(argoSdk.CreateRepositoryOpt{
					Repo:     installCmdOptions.Git.RepoUrl,
					Username: installCmdOptions.Git.Auth.Pass,
					Password: installCmdOptions.Git.Auth.Pass,
				})
				if err != nil {
					// @todo - retry url passing
					return failInstallation(fmt.Sprintf("Can't manage access to git repo: \"%s\"", err.Error()))
				}
			}

		}

		logger.Info(fmt.Sprint("Create default argocd app..."))
		err = argo.CreateDefaultApp(&argoApi)
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't create default app: \"%s\"", err.Error()))
		}

		logger.Info(fmt.Sprint("Install agent..."))
		err, _ = agentInstaller.Run(initAgentInstallOptions(&installCmdOptions))
		if err != nil {
			return failInstallation(fmt.Sprintf("Can't install argocd agent: \"%s\"", err.Error()))
		}

		successMsg := fmt.Sprintf("Successfully installed codefresh gitops controller, host: %s", argoHost)
		logger.Success(successMsg)
		eventSender := cfEventSender.New(cfEventSender.EVENT_CONTROLLER_INSTALL)
		eventSender.Success(successMsg)
		return nil
	},
}

func initAgentInstallOptions(installCmdOptions *install.CmdOptions) agentInstallPkg.InstallCmdOptions {
	var agentInstallOptions agentInstallPkg.InstallCmdOptions

	agentInstallOptions.Agent.Version = agentVersion

	agentInstallOptions.Argo.Host = installCmdOptions.Argo.Host
	agentInstallOptions.Argo.Token = installCmdOptions.Argo.Token
	agentInstallOptions.Argo.Username = installCmdOptions.Argo.Username
	agentInstallOptions.Argo.Password = installCmdOptions.Argo.Password

	agentInstallOptions.Codefresh.Host = installCmdOptions.Codefresh.Host
	agentInstallOptions.Codefresh.Token = installCmdOptions.Codefresh.Auth.Token

	agentInstallOptions.Kube.Namespace = installCmdOptions.Kube.Namespace
	agentInstallOptions.Kube.Context = installCmdOptions.Kube.Context
	agentInstallOptions.Kube.InCluster = installCmdOptions.Kube.InCluster

	agentInstallOptions.Git.Integration = installCmdOptions.Git.Integration

	agentInstallOptions.Kube.ConfigPath = installCmdOptions.Kube.ConfigPath

	agentInstallOptions.Host.HttpProxy = installCmdOptions.Host.HttpProxy
	agentInstallOptions.Host.HttpsProxy = installCmdOptions.Host.HttpsProxy

	agentInstallOptions.Codefresh.Provider = "codefresh"
	agentInstallOptions.Codefresh.SyncMode = "CONTINUE_SYNC"

	return agentInstallOptions
}

func init() {
	rootCmd.AddCommand(installCmd)
	flags := installCmd.Flags()

	flags.StringVar(&installCmdOptions.Codefresh.Host, "codefresh-host", "", "Codefresh host")
	flags.StringVar(&installCmdOptions.Codefresh.Auth.Token, "codefresh-token", "", "Codefresh api token")
	flags.StringArrayVar(&installCmdOptions.Codefresh.Clusters, "codefresh-clusters", make([]string, 0), "")

	flags.StringVar(&installCmdOptions.Argo.Token, "argo-token", "", "")
	flags.StringVar(&installCmdOptions.Argo.Host, "argo-host", "", "")
	flags.StringVar(&installCmdOptions.Argo.Username, "argo-username", "admin", "")
	flags.StringVar(&installCmdOptions.Argo.Password, "argo-password", "", "Set password for admin user of new argocd installation")

	flags.StringVar(&installCmdOptions.Kube.Namespace, "kube-namespace", "argocd", "Namespace in Kubernetes cluster")
	flags.StringVar(&installCmdOptions.Kube.ManifestPath, "install-manifest", "", "Url of argocd install manifest")
	flags.BoolVar(&installCmdOptions.Kube.InCluster, "in-cluster", false, "Set flag if Gitops controller is been installed from inside a cluster")

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

	flags.StringVar(&installCmdOptions.Host.HttpProxy, "http-proxy", "", "Http proxy")
	flags.StringVar(&installCmdOptions.Host.HttpsProxy, "https-proxy", "", "Https proxy")

	flags.BoolVar(&installCmdOptions.Controller.LoadBalancer, "load-balancer", true, "Setup load balancer")

}

func failInstallation(msg string) error {
	eventSender := cfEventSender.New(cfEventSender.EVENT_CONTROLLER_INSTALL)
	eventSender.Fail(msg)
	return errors.New(msg)
}
