package applications

import argo "github.com/codefresh-io/argocd-sdk/pkg/api"

func CreateDefault(argoApi *argo.Argo) error {
	var requestOptions argo.CreateApplicationOpt
	requestOptions.Metadata.Name = "default"
	requestOptions.Spec.Project = "default"
	requestOptions.Spec.Destination.Name = ""
	requestOptions.Spec.Destination.Namespace = ""
	requestOptions.Spec.Destination.Server = "https://kubernetes.default.svc"
	requestOptions.Spec.Source.RepoURL = "https://github.com/argoproj/argocd-example-apps.git"
	requestOptions.Spec.Source.Path = "guestbook"
	requestOptions.Spec.Source.TargetRevision = "HEAD"
	return (*argoApi).Application().CreateApplication(requestOptions)
}
