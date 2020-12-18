package kube

import (
	"context"
	"errors"
	"fmt"
	kubeobj "github.com/codefresh-io/cf-gitops-controller/pkg/kube/kubeobj"
	"github.com/codefresh-io/cf-gitops-controller/pkg/logger"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type (
	Kube interface {
		buildClient() (*kubernetes.Clientset, error)

		CreateNamespace() error
		NamespaceExists() (bool, error)
		GetArgoServerHost() (string, error)

		CreateDeployments(string) error
		UpdateDeployments(*appsv1.DeploymentList) error
		GetDeployments(string) (*appsv1.DeploymentList, error)
	}

	kube struct {
		contextName      string
		namespace        string
		pathToKubeConfig string
		clientSet        *kubernetes.Clientset
		ctx              context.Context
	}

	Options struct {
		ContextName      string
		Namespace        string
		PathToKubeConfig string
	}
)

func New(o *Options) (Kube, error) {
	client := &kube{
		contextName:      o.ContextName,
		namespace:        o.Namespace,
		pathToKubeConfig: o.PathToKubeConfig,
		ctx:              context.Background(),
	}
	clientSet, err := client.buildClient()

	if err != nil {
		return nil, err
	}

	client.clientSet = clientSet

	return client, nil
}

func (k *kube) buildClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: k.pathToKubeConfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: k.contextName,
			Context: clientcmdapi.Context{
				Namespace: k.namespace,
			},
		}).ClientConfig()

	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func (k *kube) NamespaceExists() (bool, error) {
	var exists = false
	namespace, err := k.clientSet.CoreV1().Namespaces().Get(k.namespace, metav1.GetOptions{})
	if err != nil {
		return exists, err
	}
	if namespace != nil {
		exists = true
	}
	return exists, nil
}

func (k *kube) CreateNamespace() error {
	var namespace = v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: k.namespace,
		},
	}
	_, err := k.clientSet.CoreV1().Namespaces().Create(&namespace)
	return err
}

func (k *kube) GetDeployments(labelSelector string) (*appsv1.DeploymentList, error) {
	return kubeobj.GetDeployments(k.clientSet, k.namespace, labelSelector)
}

func (k *kube) UpdateDeployments(deploymentList *appsv1.DeploymentList) error {
	for _, deployment := range deploymentList.Items {
		_, err := kubeobj.UpdateDeployment(k.clientSet, &deployment, k.namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *kube) CreateDeployments(manifestPath string) error {
	var err error
	templatesMap, err := buildTemplatesFromManifest(manifestPath)
	if err != nil {
		return err
	}
	var templatesValues map[string]interface{}

	kubeObjects, _, err := BuildObjectsFromTemplates(templatesMap, templatesValues)
	if err != nil {
		return err
	}

	kubeObjectKeys := reflect.ValueOf(kubeObjects).MapKeys()

	for _, key := range kubeObjectKeys {
		kind, name, createErr := kubeobj.CreateObject(k.clientSet, kubeObjects[key.String()], k.namespace)

		if createErr == nil {
			// skip, everything ok
		} else if statusError, errIsStatusError := createErr.(*k8sErrors.StatusError); errIsStatusError {
			if statusError.ErrStatus.Reason == metav1.StatusReasonAlreadyExists {
				logger.Warning(fmt.Sprintf("%s \"%s\" already exists", kind, name))
			} else {
				logger.Error(fmt.Sprintf("%s \"%s\" failed: %v ", kind, name, statusError))
				return statusError
			}
		} else {
			logger.Error(fmt.Sprintf("%s \"%s\" failed: %v ", kind, name, createErr))
			return createErr
		}
	}
	return err
}

func buildTemplatesFromManifest(manifestPath string) (map[string]string, error) {
	var templatesMap = map[string]string{}
	var manifestByte []byte
	var err error
	if strings.HasPrefix(manifestPath, "http://") || strings.HasPrefix(manifestPath, "https://") {
		manifestByte, err = downloadManifest(manifestPath)
	} else {
		manifestByte, err = ioutil.ReadFile(manifestPath)
	}
	if err != nil {
		return templatesMap, err
	}
	templates := strings.Split(string(manifestByte), "\n---\n")
	for n, tpl := range templates {
		templatesMap["template_"+strconv.Itoa(n)+".yaml"] = tpl
	}
	return templatesMap, err
}

func (k *kube) GetArgoServerHost() (string, error) {
	opts := metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=argocd-server"}
	svcs, err := k.clientSet.CoreV1().Services(k.namespace).List(opts)

	if err != nil {
		return "", err
	}
	if svcs == nil || len(svcs.Items) == 0 {
		return "", errors.New(fmt.Sprint("Invalid svcs"))
	}

	ingress := svcs.Items[0].Status.LoadBalancer.Ingress[0]

	if ingress.Hostname != "" {
		return "https://" + ingress.Hostname, nil
	}
	if ingress.IP != "" {
		return "https://" + ingress.IP, nil
	}

	return "", errors.New(fmt.Sprint("Can't resolve Ingress Hostname or IP"))
}

func downloadManifest(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()
	return ioutil.ReadAll(response.Body)
}
